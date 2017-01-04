//pmjq is a daemon that watches a directory and processes any file created therein
package main

// #cgo LDFLAGS: -llockfile
// #include <lockfile.h>
import "C"

import (
	"bytes"
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/mattn/go-shellwords"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
)

// NextIndex sets ix to the lexicographically next value,
// such that for each i>0, 0 <= ix[i] < lens(i).
//http://stackoverflow.com/questions/29002724/implement-ruby-style-cartesian-product-in-go
func NextIndex(ix []int, lens func(i int) int) {
	for j := len(ix) - 1; j >= 0; j-- {
		ix[j]++
		if j == 0 || ix[j] < lens(j) {
			return
		}
		ix[j] = 0
	}
}

//DirPattern bears a directory, either a pattern or a template, and, when instanciated from those to elements, a file
type DirPattern struct {
	dir      string
	pattern  *regexp.Regexp
	template *template.Template
	file     string
}

//Return the path of the file (or the tentative pattern or template if no file has been found)
func (dp DirPattern) String() string {
	if dp.pattern == nil && dp.template == nil && dp.file == "" {
		log.Fatal("I need at least one of pattern, template or file")
	}
	if dp.file != "" {
		return fmt.Sprintf("%v%v", dp.dir, dp.file)
	} else if dp.template != nil {
		return fmt.Sprintf("%v%v", dp.dir, dp.template)
	} //dp.pattern is not ""
	return fmt.Sprintf("%v%v", dp.dir, dp.pattern)
}

//FixedWidthString returns a fixed-width string representation of dp, truncated at the beginning
func (dp DirPattern) FixedWidthString() string {
	sDp := fmt.Sprintf("%v", dp)
	if len(sDp) > 20 { //FIXME: Move constant somewhere else
		return sDp[len(sDp)-20:]
	}
	return fmt.Sprintf("%-20v", sDp)
}

//PrettyFormatDirPatterns returns a concise (i.e. truncated at the beginning of each dir_pattern) fixed width representation of the given list of dir_pattern
func PrettyFormatDirPatterns(l []DirPattern) string {
	answer := "["
	for _, dp := range l {
		answer += dp.FixedWidthString()
		answer += ","
	}
	if answer[len(answer)-1] == ',' {
		answer = answer[:len(answer)-1] + "]" // Removing the last ','
	} else {
		answer += "]"
	}
	return answer
}

//Transition exposes all that is necessary to do one unit of processing
//It represents a Transition in the sense of Petri Nets theory
type Transition struct {
	//id is a unique numerical identifier for a transition (to track its path
	// for debugging purposes)
	id int

	//custodian is the name of the function that owns the structure
	custodian string

	//err is the non-remediable error that derailed this Transition's processing
	err error // FIXME: Use this to process erroneous inputs

	//inputs are the (directory, pattern, file) triplet(s) in which we look
	//for input files to feed an instance of the command with and in which
	//we store the files once we found them
	inputs []DirPattern

	//outputs are the (directory, template, file) triplet(s) in which an instance
	//of the command may write its output
	outputs []DirPattern

	//errors are the (directory, template, file) triplet(s) in which an instance
	//of the command will copy error-generating files
	errors []DirPattern

	//log_file is the (directory, pattern, file) in which an instance of the
	//command will dump its stderr
	logFile DirPattern

	//lock_release is a channel on which writing will trigger the release of one
	//locked input or output file (at random depending on the scheduler)
	//to release all locked files, write to it as many times as there are
	//files in the inputs and outputs lists together
	lockRelease chan int

	//worker_id is the id number of the worker that will launch the actual command
	workerID int //FIXME: Use this field

	//The template to be expanded to get the command to run
	cmdTemplate *template.Template

	//cmd is the Cmd structure that controls the actual execution
	cmd *exec.Cmd

	//stdin is the standard input of the process
	stdin io.WriteCloser

	//stdout is the standard output of the process
	stdout io.ReadCloser

	//stderr is the standard error of the process
	stderr io.ReadCloser

	//input_fd is the input file that should be dumped to the stdin of the process
	inputFd io.ReadCloser

	//output_fd is the output file that should be created from the process' stdout
	outputFd io.WriteCloser

	//log_fd is the output file that should be created from the process' stderr
	logFd io.WriteCloser
}

//Pretty print a transition
func (t Transition) String() string {
	//FIXME Change this comment to make it agree with the code...
	//0      Nobody     in/.* ?-cmdtemplate (????)-> out/{{}} |->log/{{}} free   msg        //Seed
	//1      dirLister in/f  ?-cmd args    (????)-> out/f    |->log/f    free   msg        //with files
	//1      locker     in/f  ?-cmd args    (????)-> out/f    |->log/f    locked msg        //with locked files
	//1      worker1    in/f  1-cmd args    (1334)-> out/f    |->log/f    locked msg        //with attributed worker
	//1      worker1    in/f  1-cmd args    (1334)-> out/f    |->log/f    locked -[0030]--> //feeding input
	//1      worker1    in/f  1-cmd args    (1334)-> out/f    |->log/f    locked --[0030]-> //getting output
	//1      worker1    in/f  1-cmd args    (1334)-> out/f    |->log/f    locked |-[0030]-> //getting stderr
	in := PrettyFormatDirPatterns(t.inputs)
	cmd := ""
	if t.cmd != nil {
		cmd = fmt.Sprintf("%v", t.cmd.Args)
	} else { // Assuming t.cmd_template != nil
		cmd = fmt.Sprintf("%v", t.cmdTemplate)
	}
	cmd = fmt.Sprintf("%-30.30v", cmd)
	pid := "??????"
	out := PrettyFormatDirPatterns(t.outputs)
	log := ""
	if t.logFile != (DirPattern{}) {
		log = t.logFile.FixedWidthString()
	}
	if t.cmd != nil {
		pid = fmt.Sprintf("%v", t.cmd.Process.Pid)
	}
	release := "free"
	if t.lockRelease != nil {
		release = "locked"
	}

	return fmt.Sprintf("%06v %-6v %v %03v-%v (%6v)-> %v %v %-11v",
		t.id, release, in, t.workerID, cmd, pid, out, log, t.custodian)
}

//These functions expose the data from a transition, as well as data about the environment
//in a way that is suitable and confortable for use in templating

//Input returns the ith file name
func (t Transition) Input(i int) string {
	return t.inputs[i].file
}

//Concurrency pattern in pmjq: workers talk to each other using channels

//candidateInputs returns a slice of all consistent input combinations
//These are the combinations where all files match their patterns, and
//where the invariant is the same among the files
func candidateInputs(seed Transition, quitEmpty bool) []Transition {
	lle := make([][]DirPattern, len(seed.inputs)) //List of list of entries, from which we will draw a cardinal product
	cardinal := 0
	for i, dp := range seed.inputs {
		entries, err := ioutil.ReadDir(dp.dir)
		if err != nil {
			log.Fatal(err)
		}
		lle[i] = make([]DirPattern, 0, len(entries))
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".lock") { //Lockfiles are not to be processed
				continue
			}
			newDp := DirPattern{dp.dir, dp.pattern.Copy(), nil, ""}
			if !newDp.pattern.MatchString(entry.Name()) { //We only add files that abide by the pattern
				continue
			}
			newDp.file = entry.Name()
			lle[i] = append(lle[i], newDp)
		}
		if cardinal == 0 {
			cardinal = len(lle[i])
		} else {
			cardinal *= len(lle[i])
		}
	}
	if quitEmpty && cardinal == 0 {
		log.Println("Nothing left to do, exiting")
		os.Exit(0)
	}
	log.Println("DBG: lle")
	log.Println(lle)
	//http://stackoverflow.com/questions/29002724/implement-ruby-style-cartesian-product-in-go
	transitions := make([]Transition, 0, cardinal)
	lens := func(i int) int { return len(lle[i]) }
	lastID := seed.id
	for ix := make([]int, len(lle)); ix[0] < lens(0); NextIndex(ix, lens) {
		t := seed
		t.id = lastID + 1
		lastID++
		t.custodian = "candidate"
		t.inputs = make([]DirPattern, 0, len(seed.inputs))
		for j, k := range ix {
			t.inputs = append(t.inputs, lle[j][k])
		}
		log.Printf("%v Candidate input", t)
		//FIXME: Before we append it, we should check the invariants
		transitions = append(transitions, t)
	}
	return transitions
}

//dirLister feeds the locker files to try to get a lock on.
//It gets those files by listing the input dir again and again, writing
//what it finds on a blocking channel
//It also provides the files to lock in the output dirs, expanding their templates
//from the name of the files in the input dirs
func dirLister(seed Transition, toLocker chan<- Transition, quitEmpty bool) {
	//log.Println("dir_lister started")
	timeChan := make(chan int)
	go func() { timeChan <- 0 }()
	lastID := seed.id
	for true {
		t := seed
		t.custodian = "dirLister"
		t.id = lastID
		//log.Println("dir_lister: Waiting on time chan")
		_ = <-timeChan
		go func() {
			time.Sleep(3 * time.Second)
			timeChan <- 0
		}()
		transitions := candidateInputs(t, quitEmpty)
		for _, t := range transitions {
			t.custodian = "dirLister"
			new_outputs := make([]DirPattern, len(t.outputs))
			copy(new_outputs, t.outputs)
			t.outputs = new_outputs
			for i := range t.outputs {
				//log.Printf("DBG1, tmplt %+v\n", tmplt)
				//log.Printf("DBG2, template data %+v\n", t)
				var b bytes.Buffer
				err := t.outputs[i].template.Execute(&b, t)
				if err != nil {
					log.Fatal(err)
				}
				t.outputs[i].file = b.String()
				//log.Printf("DBG3, outdp.file %v\n", t.outputs[0].file)
			}
			//log.Printf("DBG4, t.outputs[0].file %v\n", t.outputs[0].file)
			log.Printf("%v Output template expanded, sending to locker\n", t)
			//Feed each element to the blocking channel
			toLocker <- t
			lastID = t.id
		}
	}
}

//This function is the abort function for the locker, when something went
//during lock acquisition
func lockAbort(t Transition, waitingToken int, lockerSpawnerSynchro chan int) {
	t.custodian = "lockAbort"
	for i := 0; i < len(t.inputs)+len(t.outputs); i++ {
		log.Printf("%v Releasing partial lock %v\n", t, i)
		t.lockRelease <- 1
	}
	log.Printf("%v Giving waiting token %v back to spawner", t, waitingToken)
	lockerSpawnerSynchro <- waitingToken
}

//The locker worker tries to get a lock on the arguments provided to it by dirLister.
//It waits for the spawner to have an available slot before it tries to get a lock
//(this avoid having a lock on files and waiting, while some other machine could
//process them but does not have the lock)
//The spawner tells the locker that a slot is available by writing on channel
//lockerSpawner_synchro.
//If it can not get a lock on all the files, the locker signals so by sending a token
//back to the spawner through lockerSpawner synchro.
//Once all arguments to a call have been locked, it passes them on to the spawner
//via the toSpawner channel.
func locker(fromDirLister <-chan Transition, lockerSpawnerSynchro chan int,
	toSpawner chan<- Transition) {
	//log.Println("locker started")
	for true {
		//log.Println("locker: Waiting on dirLister to suggest files to try to lock:")
		t := <-fromDirLister
		t.custodian = "locker"
		log.Printf("%v Received from dirLister", t)
		success := make(chan int)
		t.lockRelease = make(chan int)
		nbFiles := len(t.inputs) + len(t.outputs)
		//files := make([]DirPattern, 0, len(t.inputs)+len(t.outputs))
		//files = append(files, t.inputs...)
		//files = append(files, t.outputs...)
		//log.Println("locker: Waiting for spawner to be ready to spawn")
		log.Printf("%v waiting on spawner", t)
		waitingToken := <-lockerSpawnerSynchro //Will unblock once spawner is ready to spawn
		log.Printf("%v Got waiting token %v from spawner", t, waitingToken)
		for i := 0; i < nbFiles; i++ {
			go lockFile(t, i, success, t.lockRelease)
		}
		status := 0
		for i := 0; i < nbFiles; i++ {
			status += <-success
			log.Printf("%v After iteration %v, status is %v", t, i, status)
		}
		if status != 0 { //At least one lock was not acquired
			lockAbort(t, waitingToken, lockerSpawnerSynchro)
			continue
		}
		//All locks acquired
		status = 0
		for _, dp := range t.inputs {
			if _, err := os.Stat(dp.String()); os.IsNotExist(err) {
				//If file does not exist
				status = -1
				break
			}
		}
		if status != 0 { //At least one file no longer exists
			lockAbort(t, waitingToken, lockerSpawnerSynchro)
			continue
		}
		//All files exist
		log.Printf("%v Sending locked files to spawner", t)
		//logTransition(t)
		toSpawner <- t
	}
}

//The lockFile function creates a lock on the given file.
//It defers the removal of the lock.
//It refreshes the lock every minute (see the man page for lockfile_create).
//It exits only when something is written to the release channel.
//It writes its status (0:success, !=0: failure) on the success channel.
func lockFile(t Transition, fileno int, success chan<- int, release <-chan int) {
	t.custodian = "lockFile"
	var fname string
	if fileno < len(t.inputs) {
		fname = fmt.Sprintf("%v", t.inputs[fileno])
	} else {
		fname = fmt.Sprintf("%v", t.outputs[fileno-len(t.inputs)])
	}
	fname += ".lock"
	log.Printf("%v Acquiring lock on %v", t, fname)
	errInt := int(C.lockfile_create(C.CString(fname), 0, 0))
	if errInt != 0 {
		log.Printf("%v Could not get a lock on %v error nb %v", t, fname, errInt)
		success <- errInt
		i := <-release
		log.Printf("%v %v exiting status %v", t, fname, i)
		return
	}
	success <- 0
	defer func() {
		errInt = int(C.lockfile_remove(C.CString(fname)))
		log.Printf("%v Deferred lock release on %v: %v", t, fname, errInt)
	}()
	timeChan := make(chan int)
	go func() { timeChan <- 0 }()
	for true {
		select {
		case _ = <-timeChan:
			log.Printf("%v Refreshing lock on %v ", t, fname)
			C.lockfile_touch(C.CString(fname))
			go func() {
				time.Sleep(60 * time.Second)
				timeChan <- 0
			}()
		case i := <-release:
			log.Printf("%v %v exiting status %v", t, fname, i)
			return
		}
	}
}

// Read from src in buckets of 4096 and dumps them in dst
// It logs everything in the process, appending logid before
func goBucketDumper(t Transition, srcdst string) chan error {
	c := make(chan error)
	var src io.ReadCloser
	var dst io.WriteCloser
	t.custodian += " " + srcdst
	if srcdst == "disk->stdin" {
		src = t.inputFd
		dst = t.stdin
	} else if srcdst == "stdout->disk" {
		src = t.stdout
		dst = t.outputFd
	} else if srcdst == "stderr->disk" {
		src = t.stderr
		dst = t.logFd
	}
	go func() {
		data := make([]byte, 4096)
		for true {
			data = data[:cap(data)]
			n, err := src.Read(data)
			if err != nil {
				if err == io.EOF {
					src.Close()
					dst.Close()
					c <- nil
					break
				} else {
					log.Fatal(err)
					c <- err
					break
				}
			}
			data = data[:n]
			if srcdst == "disk->stdin" {
				log.Printf("%v --[%04v]", t, n)
			} else if srcdst == "stdout->disk" {
				log.Printf("%v [%04v]-->", t, n)
			} else if srcdst == "stderr->disk" {
				log.Printf("%v [%04v]-|", t, n)
			}
			start := 0
			for start < n {
				n2, err := dst.Write(data)
				if err != nil {
					log.Fatal(err)
					c <- err
					break
				}
				start += n2
			}
		}
	}()
	return c
}

//The actualWorkers are tasked with launching and monitoring the data processing tasks.
//They receive their transition as an argument, and they output their id
//on the outputChannel once they are done.
func actualWorker(t Transition, id int, outputChannel chan<- int) {
	t.custodian = fmt.Sprintf("worker%v", id)
	t.workerID = id
	log.Printf("%v Starting\n", t)
	//Expand the command
	var b bytes.Buffer
	err := t.cmdTemplate.Execute(&b, t)
	if err != nil {
		log.Fatal(err)
	}
	cmdArgv, err := shellwords.Parse(b.String())
	//Launch the process
	t.cmd = exec.Command(cmdArgv[0], cmdArgv[1:]...)
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}
	defer stderr.Close()
	err = t.cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	//Wait for the data
	t.stdin = stdin
	t.stdout = stdout
	t.stderr = stderr
	log.Printf("%v Command started \n", t)
	//Launch a worker that reads from disk and writes to the stdin of the command
	//Wrapping it in an anonymous func so that Close() is called as soon
	//as we are finished with the FDs
	// http://grokbase.com/t/gg/golang-nuts/134883hv3h/go-nuts-io-closer-and-closing-previously-closed-object
	if err := func() error {
		t.inputFd, err = os.Open(t.inputs[0].String())
		if err != nil {
			log.Fatal(err)
		}
		defer t.inputFd.Close()
		d2schan := goBucketDumper(t, "disk->stdin")
		//Launch a worker that reads from the command and writes to disk
		t.outputFd, err = os.Create(t.outputs[0].String())
		if err != nil {
			log.Fatal(err)
		}
		defer t.outputFd.Close()
		s2dchan := goBucketDumper(t, "stdout->disk")
		//Launch a worker that reads from the command's stderr and logs it
		var e2dchan chan error
		if t.logFile != (DirPattern{}) {
			var b bytes.Buffer
			err := t.logFile.template.Execute(&b, t)
			if err != nil {
				log.Fatal(err)
			}
			t.logFile.file = b.String() //FIXME: Tout ça doit se trouver dans une fonction de DirPattern
			t.logFd, err = os.Create(t.logFile.String())
			if err != nil {
				log.Fatal(err)
			}
			defer t.logFd.Close()
			e2dchan = goBucketDumper(t, "stderr->disk")
		}
		//Wait for it to finish
		log.Printf("%v Waiting for job to finish", t)
		<-d2schan
		<-s2dchan
		if e2dchan != nil {
			<-e2dchan
		}
		return t.cmd.Wait()
	}(); err != nil {
		if t.errors == nil {
			log.Fatal(err)
		}
		//Move the input files to the error_dirs
		for i := range t.errors {
			var b bytes.Buffer
			err := t.errors[i].template.Execute(&b, t)
			if err != nil {
				log.Fatal(err)
			}
			t.errors[i].file = b.String()
			err = os.Rename(t.inputs[i].String(), t.errors[i].String())
			if err != nil {
				log.Fatal(err)
			}
		}
		// //Remove the (probably incomplete, maybe nonexisting) output file
		for i := range t.outputs {
			os.Remove(t.outputs[i].String())
		}
	} else {
		//Remove the file from the input folder
		err = os.Remove(t.inputs[0].String())
		if err != nil {
			log.Fatal(err)
		}
	}
	//Release the file locks
	t.lockRelease <- 0
	t.lockRelease <- 0
	outputChannel <- id
}

//The spawner worker has a few slots for actual_workers to be launched.
//actual_worker instances are given an input and output channel.
//They read the arguments on the input channel.
//These are given a list of arguments on their input channel as read from locker.
//They send their id on the common output channel when they are done.
func spawner(seed Transition,
	lockerSpawnerSynchro chan int, fromLocker <-chan Transition, nbSlots int) {
	//log.Println("spawner started")
	availableWorkers := make(chan int)
	//Add the available workers to the Queue
	for i := 0; i < nbSlots; i++ {
		go func(j int) { availableWorkers <- j }(i)
	}
	i := <-availableWorkers
	t := seed
	t.custodian = "spawner"
	for true {
		log.Printf("%v Waiting for an available worker", t)
		i = <-availableWorkers
		log.Printf("%v worker %v waiting on locker\n", t, i)
		lockerSpawnerSynchro <- i //Signal locker that we are ready
		//to work by sending it a waiting token
		select {
		case i = <-lockerSpawnerSynchro: //Locker gives us our token back: it could
			//not get the locks
			log.Printf("%v Locker is sending token %v back to the pool\n", t, i)
			go func(j int) { availableWorkers <- j }(i)
		case t := <-fromLocker:
			t.custodian = "spawner"
			log.Printf("%v Assigning to worker %v\n", t, i)
			go actualWorker(t, i, availableWorkers) //Launch the actual worker
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	usage := `pmjq.

	Usage: pmjq  [--quit-when-empty] --input=<inpattern>... [--invariant=<re_template>] <cmdtemplate> --output=<outtemplate>... [--stderr=<logtemplate>] [--error=<errortemplate>...]
	       pmjq -h | --help
	       pmjq --version

  Options:
     --help -h                  Show this message
     --version                  Show version information and exit
     --quit-when-empty          Exit with 0 status when the input dir is empty
     --input=<inpattern>        The regex a file must match in order to be processed
     --invariant=<re_template>  Must only be specified if multiple input patterns are passed. Iff the regex template expansion is the same for all --input matches, the matching files are processed together.
     --output=<outtemplate>     The name of the output file(s) are the expansion of this(ese) template(s), using the DSL of Golang's text/template. Templates ending in / when there is only one input and one output will result in the input file's name being used as the output file's name.
     --stderr=<logtemplate>     The name of the log file where each instance of cmd will dump it stderr is the expansion of this template. Templates ending in / will result in the first input file's name being used as the log file's name.
     --error=<error-dir>        If specified, there must be as many as there are --input. If specified, pmjq does not crash on error but move the incriminated file(s) to their new name(s) given by the expansion of these template(s). Templates ending in / when there is only one input and one output will result in the input file's name being used as the error file's name.
`
	arguments, err := docopt.Parse(usage, nil, true, "Poor Man's Job Queue, v 1.0.0", false)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("pmjq started")
	log.Println(arguments)
	seed := Transition{
		id:          0,
		custodian:   "Seed",
		inputs:      make([]DirPattern, 0, len(arguments["--input"].([]string))),
		outputs:     make([]DirPattern, 0, len(arguments["--output"].([]string))),
		cmdTemplate: template.Must(template.New("Command").Parse(arguments["<cmdtemplate>"].(string))),
	}
	log.Printf("%v Initial seed\n", seed)
	for _, inpattern := range arguments["--input"].([]string) {
		dir, pattern := filepath.Split(inpattern)
		if pattern == "" {
			pattern = ".*" //Unspecified pattern defaults to all files
		}
		seed.inputs = append(seed.inputs, DirPattern{dir, regexp.MustCompile(pattern), nil, ""})
	}
	log.Printf("%v Input patterns\n", seed)
	for _, outtemplate := range arguments["--output"].([]string) {
		dir, tmplt := filepath.Split(outtemplate)
		if tmplt == "" {
			tmplt = "{{.Input 0}}" //Unspecified template defaults to same name as first input file
		}
		seed.outputs = append(seed.outputs, DirPattern{dir, nil, template.Must(template.New("One of the outputs").Parse(tmplt)), ""})
	}
	log.Printf("%v Output templates\n", seed)
	if len(arguments["--error"].([]string)) > 0 {
		for _, errtemplate := range arguments["--error"].([]string) {
			dir, tmplt := filepath.Split(errtemplate)
			if tmplt == "" {
				tmplt = "{{.Input 0}}" //Unspecified template defaults to same name as first input file
			}
			seed.errors = append(seed.errors, DirPattern{dir, nil, template.Must(template.New("One of the errors").Parse(tmplt)), ""})
		}
	}
	if arguments["--stderr"] != nil {
		dir, tmplt := filepath.Split(arguments["--stderr"].(string))
		if tmplt == "" {
			tmplt = "{{.Input 0}}" //Unspecified template defaults to same name as first input file
		}
		seed.logFile = DirPattern{dir, nil, template.Must(template.New("The log file").Parse(tmplt)), ""}
	}
	// cmd_argv, err := shellwords.Parse(arguments["<filter>"].(string))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	fromDirListerToLocker := make(chan Transition)
	go dirLister(seed, fromDirListerToLocker, arguments["--quit-when-empty"].(bool))
	fromLockerToSpawner := make(chan Transition)
	lockerSpawnerSynchro := make(chan int)
	go locker(fromDirListerToLocker, lockerSpawnerSynchro, fromLockerToSpawner)
	go spawner(seed, lockerSpawnerSynchro, fromLockerToSpawner, 4)

	time.Sleep(3 * time.Second)
	//log.Println("Exiting.")
	c := make(chan int)
	<-c
}
