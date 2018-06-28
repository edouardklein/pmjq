//pmjq is a daemon that watches a directory and processes any file created therein
package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/mattn/go-shellwords"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//RandomNonce is the (hopefully) unique indentifier of a particular instance of pmjq
var RandomNonce = fmt.Sprintf("%v", rand.Int())

//lockFileTouch changes the content of the file to avoid it being
//detected as stale
func lockFileTouch(name string) error {
	b, err := ioutil.ReadFile(name)
	if err != nil {
		return err
	}
	i, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}
	return ioutil.WriteFile(name, []byte(fmt.Sprintf("%v", i+1)), 0755)
}

//lockFileRemoveIfStale will delete a lock file if its
// contents stay unchanged for 2 minutes
func lockFileRemoveIfStale(name string) {
	b, err := ioutil.ReadFile(name)
	if err != nil {
		return
	}
	time.Sleep(120 * time.Second)
	bb, err := ioutil.ReadFile(name)
	if err != nil {
		return
	}
	if bytes.Compare(b, bb) == 0 {
		log.Println("WARNING: Removing stale lock file " + name)
		os.Remove(name)
	}
}

//lockFileActuallyCreate does the actual file creation dirty work
//We can not trust all filesystems to respect mutual exclusion or
//atomicity, that civilized people respect, so we code for
//the lowest common denominator, using simple primitives :
//If any write access is performed with this function by another
//process, then one of the two process should return an error
func lockFileActuallyCreate(name string) error {
	fd, err := os.Create(name)
	if err != nil {
		return err
	}
	fd.WriteString(RandomNonce)
	fd.Close()
	b, err := ioutil.ReadFile(name)
	if err != nil {
		return err
	}
	if string(b) != RandomNonce {
		return errors.New("File " + name + " was changed from under us")
	}
	return nil
}

//lockFileCreate tries to create the given lock file
func lockFileCreate(name string) error {
	//Check existence
	if _, err := os.Stat(name); os.IsNotExist(err) {
		//If it does not exist
		//Try to create it
		return lockFileActuallyCreate(name)
	} else if err != nil {
		log.Fatal(err)
	}
	//If it exists
	//Check for staleness and return an error
	go lockFileRemoveIfStale(name)
	return errors.New("Lock file already exists")
}

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

//DirPattern bears a directory and a pattern to look for in the file names
// of this directory
type DirPattern struct {
	dir           string
	patternString string
	pattern       regexp.Regexp
}

//Return the tentative pattern
func (dp *DirPattern) String() string {
	return fmt.Sprintf("%v%v", dp.dir, dp.patternString)
}

//DirTemplate bears a directory and a template to expand when creating
// new files in this directory
type DirTemplate struct {
	dir            string
	templateString string
	template       template.Template
}

func (dt *DirTemplate) isNull() bool {
	return dt.templateString == ""
}

//Return the tentative template
func (dt *DirTemplate) String() string {
	return fmt.Sprintf("%v%v", dt.dir, dt.templateString)
}

//ExecWithTransition returns the path of the receiver when exectued
// whithin the given transition
func (dt *DirTemplate) ExecWithTransition(t *Transition) string {
	var b bytes.Buffer
	err := dt.template.Execute(&b, t)
	if err != nil {
		log.Fatal(err)
	}
	return path.Join(dt.dir, b.String())
}

//FixedWidthString returns a fixed-width string representation of x,
// truncated at the beginning
func FixedWidthString(x string) string {
	answer := x
	if len(answer) > 20 { //FIXME: Move constant somewhere else
		return answer[len(answer)-20:]
	}
	return fmt.Sprintf("%-20v", answer)
}

//FixedWidthStrings returns a concise (i.e. truncated at the beginning of each)
// fixed width representation of the given list of strings
func FixedWidthStrings(l []string) string {
	answer := "["
	for i := range l {
		answer += FixedWidthString(l[i])
		answer += ", "
	}
	if answer[len(answer)-2:] == ", " {
		answer = answer[:len(answer)-2] + "] " // Removing the last ','
	} else {
		answer += "]"
	}
	return answer
}

//DTFixedWidthStrings returns a fixed witdth representation
// of the given slice of DirTemplates
func DTFixedWidthStrings(l []*DirTemplate) string {
	strings := make([]string, len(l))
	for i, dt := range l {
		strings[i] = fmt.Sprintf("%v", dt)
	}
	return FixedWidthStrings(strings)
}

//DPFixedWidthStrings returns a fixed witdth representation
// of the given slice of DirPatterns
//FIXME: Avoid the redundancy in the code of all
//*FixedWidth functions
func DPFixedWidthStrings(l []*DirPattern) string {
	strings := make([]string, len(l))
	for i, dt := range l {
		strings[i] = fmt.Sprintf("%v", dt)
	}
	return FixedWidthStrings(strings)
}

//Transition exposes all that is necessary to do one unit of processing
//It represents a Transition in the sense of Petri Nets theory
type Transition struct {
	//id is a unique numerical identifier for a transition (to track its path
	// for debugging purposes). No two transitions should bear the same id
	id int

	//custodian is the name of the function that owns the structure
	custodian string

	//inputPatterns are the (directory, pattern) couple(s) in which we look
	//for input files to feed an instance of the command with
	inputPatterns []*DirPattern

	//inputFiles is the list of filenames to be processed
	inputFiles []string

	//inputPaths is the list of paths (dir + name) to be processed
	//those paths are considered as understandable by the command that
	//will be launched.
	//FIXME: Add a 'cwd' parameter to a transition to allow
	//for a working directory to be specified
	//FIXME: Add a 'env' parameter to a transition to allow
	//the user to specify environment variables
	inputPaths []string

	//invariantTemplate is what will be expanded to make the Invariant
	//after the input file names are matched
	invariantTemplate string

	//Invariant is the common part of all input file names
	Invariant string

	//NamedMatches maps the name of a subgroup its value in the last input file
	//name that matched a regex with a subgroup of this name
	NamedMatches map[string]string

	//outputs are the (directory, template) couple(s) in which an instance
	//of the command may write its output
	outputTemplates []*DirTemplate

	//outputPaths is the list of paths in which some output may be written
	outputPaths []string

	//errors are the (directory, template) couple(s) in which an instance
	//of the command will copy error-generating files
	errorTemplates []*DirTemplate

	//errorPaths is the list of paths in which the inputs will be copied
	//in case of an error
	errorPaths []string

	//logTemplate is the (directory, template) in which an instance of the
	//command will dump its stderr
	logTemplate *DirTemplate

	//logPath is the path of the file in which we dump stderr
	logPath string

	//lock_release is a channel on which writing will trigger the release of one
	//locked input or output file (at random depending on the scheduler)
	//to release all locked files, write to it as many times as there are
	//files in the inputs and outputs lists together
	lockRelease chan int

	//workerID is the id number of the worker that will launch the actual command
	workerID int

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

	//inputFd is the input file fd that should be dumped to the stdin of the process
	inputFd io.ReadCloser

	//outputFd is the output file fd that should be created from the process' stdout
	outputFd io.WriteCloser

	//logFd is the output file fd that should be created from the process' stderr
	logFd io.WriteCloser
}

//Sapling duplicates a seed transition,
// return the copy and increase the seed's ID so that
// no two duplicate share the same ID
func (t *Transition) Sapling() Transition {
	answer := *t
	t.id++
	return answer
}

//Pretty print a transition
func (t *Transition) String() string {
	//FIXME Change this comment to make it agree with the code...
	//0      Nobody     in/.* ?-cmdtemplate (????)-> out/{{}} |->log/{{}} free   msg        //Seed
	//1      dirLister in/f  ?-cmd args    (????)-> out/f    |->log/f    free   msg        //with files
	//1      locker     in/f  ?-cmd args    (????)-> out/f    |->log/f    locked msg        //with locked files
	//1      worker1    in/f  1-cmd args    (1334)-> out/f    |->log/f    locked msg        //with attributed worker
	//1      worker1    in/f  1-cmd args    (1334)-> out/f    |->log/f    locked -[0030]--> //feeding input
	//1      worker1    in/f  1-cmd args    (1334)-> out/f    |->log/f    locked --[0030]-> //getting output
	//1      worker1    in/f  1-cmd args    (1334)-> out/f    |->log/f    locked |-[0030]-> //getting stderr
	var in string
	if len(t.inputPaths) > 0 {
		in = FixedWidthStrings(t.inputPaths)
	} else {
		in = DPFixedWidthStrings(t.inputPatterns)
	}
	var cmd string
	if t.cmd != nil {
		cmd = fmt.Sprintf("%v", t.cmd.Args)
	} else { // Assuming t.cmdTemplate != nil
		cmd = fmt.Sprintf("%v", t.cmdTemplate)
	}
	cmd = fmt.Sprintf("%-30.30v", strings.Replace(cmd, "\n", " ", -1))
	pid := "??????"
	var out string
	if len(t.outputPaths) > 0 {
		out = FixedWidthStrings(t.outputPaths)
	} else {
		out = DTFixedWidthStrings(t.outputTemplates)
	}
	var log string
	if t.logPath != "" {
		log = FixedWidthString(t.logPath)
	} else if t.logTemplate != nil {
		log = FixedWidthString(fmt.Sprintf("%v", t.logTemplate))
	}
	if t.cmd != nil {
		pid = fmt.Sprintf("%v", t.cmd.Process.Pid)
	}
	release := "free"
	if t.lockRelease != nil {
		release = "locked"
	}

	return fmt.Sprintf("%06v %-6v %v %03v-%v (%6v)-> %v [%v] %-11v",
		t.id, release, in, t.workerID, cmd, pid, out, log, t.custodian)
}

//These functions expose the data from a transition, as well as data about the environment
//in a way that is suitable and confortable for use in templating

//Input returns the ith file name
func (t *Transition) Input(i int) string {
	return t.inputFiles[i]
}

//minInt return the minimum value among all its int arguments
func minInt(li ...int) int {
	m := li[0]
	for _, val := range li[1:] {
		if val < m {
			m = val
		}
	}
	return m
}

//candidateInputs returns a slice of all consistent input combinations
//These are the combinations where all files match their patterns, and
//where the invariant is the same among the files
func candidateInputs(seed *Transition, quitEmpty bool) chan *Transition {
	lle := make([][]string, len(seed.inputPatterns))  //List of list of entries, from which we will draw a cartesian product
	cardinal := 0                                     // Number of elements in the cartesian product
	waiting := make([]int, len(seed.inputPatterns))   //Number of files in each dir
	inputs := make([]string, len(seed.inputPatterns)) //Names of the dirs
	for i := range seed.inputPatterns {
		inputs[i] = seed.inputPatterns[i].dir
	}
	for i := range seed.inputPatterns {
		entries, err := ioutil.ReadDir(seed.inputPatterns[i].dir)
		if err != nil {
			log.Fatal(err)
		}
		lle[i] = make([]string, 0, len(entries))
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".lock") { //Lockfiles are not to be processed
				continue
			}
			if !seed.inputPatterns[i].pattern.MatchString(entry.Name()) {
				//We only add files that abide by the pattern
				continue
			}
			lle[i] = append(lle[i], entry.Name())
		}
		if cardinal == 0 {
			cardinal = len(lle[i])
		} else {
			cardinal *= len(lle[i])
		}
		waiting[i] = len(lle[i])
	}
	if quitEmpty && cardinal == 0 {
		log.Println("Nothing left to do, exiting")
		os.Exit(0)
	}
	mi := minInt(waiting...)
	log.Printf("%v INFO Candidates for %v:%v", seed, strings.Join(inputs, ":"), mi)
	//log.Println("DBG: lle")
	//log.Println(lle)
	//http://stackoverflow.com/questions/29002724/implement-ruby-style-cartesian-product-in-go
	transitions := make(chan *Transition)
	go func() {
		lens := func(i int) int { return len(lle[i]) }
		// Quitting early if any of the set is empty
		for i := 0; i < len(lle); i++ {
			if lens(i) == 0 {
				close(transitions)
				return
			}
		}
	transitionAccumulation:
		for ix := make([]int, len(lle)); ix[0] < lens(0); NextIndex(ix, lens) {
			// ix refers to an element of the cartesian product
			// it is an array of length len(seed.inputPatterns)
			// Each element is the index of the entry
			t := seed.Sapling()
			t.custodian = "candidate"
			t.NamedMatches = make(map[string]string)
			t.inputFiles = make([]string, 0, len(t.inputPatterns))
			t.inputPaths = make([]string, 0, len(t.inputPatterns))
			for j, k := range ix {
				//j is the index in [0:len(seed.inputPatterns)]
				//k is the index in the j-th list
				currentEntry := lle[j][k]
				currentPattern := &t.inputPatterns[j].pattern
				currentPath := path.Join(t.inputPatterns[j].dir,
					currentEntry)
				t.inputFiles = append(t.inputFiles, currentEntry)
				t.inputPaths = append(t.inputPaths, currentPath)
				matchInts := currentPattern.FindStringSubmatchIndex(currentEntry)
				invariant := string(currentPattern.ExpandString(make([]byte, 0), t.invariantTemplate, currentEntry, matchInts))
				if t.Invariant == "" {
					t.Invariant = invariant
				} else if t.Invariant != invariant {
					continue transitionAccumulation //We stop building a candidate as soon as we see the invariants don't match
				}
				//http://stackoverflow.com/questions/20750843/using-named-matches-from-go-regex
				match := currentPattern.FindStringSubmatch(currentEntry)
				for i, name := range currentPattern.SubexpNames() {
					if i != 0 {
						t.NamedMatches[name] = match[i]
					}
				}
			}
			log.Printf("%v DEBUG Candidate input", &t)
			transitions <- &t
		}
		close(transitions)
	}()
	return transitions
}

//dirLister feeds the locker files to try to get a lock on.
//It gets those files by listing the input dir again and again, writing
//what it finds on a blocking channel
//It also provides the files to lock in the output dirs, expanding their templates
//from the name of the files in the input dirs
func dirLister(seed *Transition, toLocker chan<- *Transition, quitEmpty bool) {
	//log.Println("dir_lister started")
	timeChan := make(chan int)
	go func() { timeChan <- 0 }()
	for true {
		t := seed.Sapling()
		t.custodian = "dirLister"
		_ = <-timeChan
		go func() {
			time.Sleep(3 * time.Second)
			timeChan <- 0
		}()
		transitions := candidateInputs(&t, quitEmpty)
		for t := range transitions {
			t.custodian = "dirLister"
			t.outputPaths = make([]string, len(t.outputTemplates))
			for i := range t.outputTemplates {
				log.Printf("%v DEBUG i is %v, out of %v and %v\n", t, i, len(t.outputTemplates), len(t.outputPaths))
				t.outputPaths[i] = t.outputTemplates[i].ExecWithTransition(t)
			}
			log.Printf("%v DEBUG Output template expanded, sending to locker\n", t)
			//Feed each element to the blocking channel
			toLocker <- t
		}
	}
}

//This function is the abort function for the locker, when something went
//during lock acquisition
func lockAbort(t *Transition, waitingToken int, lockerSpawnerSynchro chan int) {
	t.custodian = "lockAbort"
	for i := 0; i < len(t.inputPaths)+len(t.outputPaths); i++ {
		log.Printf("%v DEBUG Releasing partial lock %v\n", t, i)
		t.lockRelease <- 1
	}
	log.Printf("%v DEBUG Giving waiting token %v back to spawner", t, waitingToken)
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
func locker(fromDirLister <-chan *Transition, lockerSpawnerSynchro chan int,
	toSpawner chan<- *Transition) {
	//log.Println("locker started")
	for true {
		//log.Println("locker: Waiting on dirLister to suggest files to try to lock:")
		t := <-fromDirLister
		t.custodian = "locker"
		log.Printf("%v DEBUG Received from dirLister", t)
		success := make(chan int)
		t.lockRelease = make(chan int)
		nbFiles := len(t.inputPaths) + len(t.outputPaths)
		log.Printf("%v DEBUG waiting on spawner", t)
		waitingToken := <-lockerSpawnerSynchro //Will unblock once spawner is ready to spawn
		log.Printf("%v DEBUG Got waiting token %v from spawner", t, waitingToken)
		for i := 0; i < nbFiles; i++ {
			go lockFile(t, i, success, t.lockRelease)
		}
		status := 0
		for i := 0; i < nbFiles; i++ {
			status += <-success
			log.Printf("%v DEBUG After iteration %v, status is %v", t, i, status)
		}
		if status != 0 { //At least one lock was not acquired
			lockAbort(t, waitingToken, lockerSpawnerSynchro)
			continue
		}
		//All locks acquired
		status = 0
		log.Printf("%v DEBUG All locks acquired, testing existence", t)
		for _, fname := range t.inputPaths {
			log.Printf("%v DEBUG All locks acquired, testing existence of %v", t, fname)
			if _, err := os.Stat(fname); os.IsNotExist(err) {
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
		log.Printf("%v DEBUG Sending locked files to spawner", t)
		toSpawner <- t
	}
}

//The lockFile function creates a lock on the given file.
//It defers the removal of the lock.
//It refreshes the lock every minute
//It exits only when something is written to the release channel.
//It writes its status (0:success, !=0: failure) on the success channel.
func lockFile(t *Transition, fileno int, success chan<- int, release <-chan int) {
	t.custodian = "lockFile"
	var fname string
	if fileno < len(t.inputPaths) {
		fname = fmt.Sprintf("%v", t.inputPaths[fileno])
	} else {
		fname = fmt.Sprintf("%v", t.outputPaths[fileno-len(t.inputPaths)])
	}
	fname += ".lock"
	log.Printf("%v DEBUG Acquiring lock on %v", t, fname)
	err := lockFileCreate(fname)
	if err != nil {
		log.Printf("%v WARNING Could not get a lock on %v error %v", t, fname, err)
		success <- 1
		i := <-release
		log.Printf("%v DEBUG %v exiting status %v", t, fname, i)
		return
	}
	success <- 0
	defer func() {
		err = os.Remove(fname)
		log.Printf("%v DEBUG Deferred lock release on %v: %v", t, fname, err)
	}()
	timeChan := make(chan int)
	go func() { timeChan <- 0 }()
	for true {
		select {
		case _ = <-timeChan:
			log.Printf("%v DEBUG Refreshing lock on %v ", t, fname)
			lockFileTouch(fname)
			go func() {
				time.Sleep(60 * time.Second)
				timeChan <- 0
			}()
		case i := <-release:
			log.Printf("%v DEBUG %v exiting status %v", t, fname, i)
			return
		}
	}
}

// Read from src in buckets of 4096 and dumps them in dst
// It logs everything in the process, appending logid before
func goBucketDumper(t *Transition, srcdst string) chan error {
	c := make(chan error)
	var src io.ReadCloser
	var dst io.WriteCloser
	if srcdst == "disk->stdin" {
		log.Printf("%v DEBUG %v, Input file is %v",
			t, srcdst, t.inputFd)
		src = t.inputFd
		dst = t.stdin
	} else if srcdst == "stdout->disk" {
		log.Printf("%v DEBUG %v, Input file is %v",
			t, srcdst, t.stdout)
		src = t.stdout
		dst = t.outputFd
	} else if srcdst == "stderr->disk" {
		log.Printf("%v DEBUG %v, Input file is %v",
			t, srcdst, t.stderr)
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
			log.Printf("%v DEBUG %v --[%04v]->", t, srcdst, n)
			start := 0
			for start < n {
				n2, err := dst.Write(data)
				if err != nil {
					log.Printf("%v In bucketdumper, start=%v, n=%v, n2=%v.", t, start, n, n2)
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
func actualWorker(t *Transition, id int, outputChannel chan<- int) {
	t.custodian = fmt.Sprintf("worker%v", id)
	t.workerID = id
	log.Printf("%v DEBUG Starting\n", t)
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
	log.Printf("%v DEBUG Command started \n", t)
	//Launch a worker that reads from disk and writes to the stdin of the command
	//Wrapping it in an anonymous func so that Close() is called as soon
	//as we are finished with the FDs
	// http://grokbase.com/t/gg/golang-nuts/134883hv3h/go-nuts-io-closer-and-closing-previously-closed-object
	if err := func() error {
		d2schan := make(chan error)
		if len(t.inputPaths) == 1 {
			t.inputFd, err = os.Open(t.inputPaths[0])
			log.Printf("%v DEBUG Input file just opened %v", t, t.inputFd)
			if err != nil {
				log.Fatal(err)
			}
			defer t.inputFd.Close()
			d2schan = goBucketDumper(t, "disk->stdin")
		} else {
			// With multiple inputs, we don't write to the command's stdin
			go func() {
				t.stdin.Close() //Explicitely prevent input on stdin
				d2schan <- nil
			}()
		}
		//Launch a worker that reads from the command and writes to disk
		log.Printf("%v DEBUG Actual worker CP 1", t)
		s2dchan := make(chan error)
		if len(t.outputPaths) == 1 {
			t.outputFd, err = os.Create(t.outputPaths[0])
			if err != nil {
				log.Fatal(err)
			}
			defer t.outputFd.Close()
			s2dchan = goBucketDumper(t, "stdout->disk")
		} else {
			// With multiple outputs, we don't write the command's stdout
			// (FIXME: This output is lost)
			go func() {
				s2dchan <- nil
			}()
		}
		//Launch a worker that reads from the command's stderr and logs it
		log.Printf("%v DEBUG Actual worker CP 2", t)
		var e2dchan chan error
		if t.logTemplate != nil {
			t.logPath = t.logTemplate.ExecWithTransition(t)
			t.logFd, err = os.Create(t.logPath)
			if err != nil {
				log.Fatal(err)
			}
			defer t.logFd.Close()
			e2dchan = goBucketDumper(t, "stderr->disk")
		}
		//Wait for it to finish
		log.Printf("%v DEBUG Waiting for job to finish", t)
		<-d2schan
		<-s2dchan
		if e2dchan != nil {
			<-e2dchan
		}
		return t.cmd.Wait()
	}(); err != nil {
		if t.errorTemplates == nil {
			log.Fatal(err)
		}
		//Move the input files to the error_dirs
		t.errorPaths = make([]string, len(t.errorTemplates))
		for i := range t.errorTemplates {
			t.errorPaths[i] = t.errorTemplates[i].ExecWithTransition(t)
			err = os.Rename(t.inputPaths[i], t.errorPaths[i])
			log.Printf("%v ERROR Rejected file from %v to %v (%v)",
				t, t.inputPaths[i],
				t.errorPaths[i],
				t.logPath)
			if err != nil {
				log.Fatal(err)
			}
		}
		// //Remove the (probably incomplete, maybe nonexisting) output file
		for i := range t.outputPaths {
			os.Remove(t.outputPaths[i])
		}
	} else {
		//Remove the file from the input folder
		for _, fname := range t.inputPaths {
			err = os.Remove(fname)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	//Release the file locks
	for i := 0; i < len(t.inputPaths)+len(t.outputPaths); i++ {
		log.Printf("%v DEBUG Releasing lock %v\n", t, i)
		t.lockRelease <- 0
	}
	outputChannel <- id
}

//The spawner worker has a few slots for actual_workers to be launched.
//actual_worker instances are given an input and output channel.
//They read the arguments on the input channel.
//These are given a list of arguments on their input channel as read from locker.
//They send their id on the common output channel when they are done.
func spawner(seed *Transition,
	lockerSpawnerSynchro chan int, fromLocker <-chan *Transition,
	nbSlots int) {
	//log.Println("spawner started")
	availableWorkers := make(chan int)
	//Add the available workers to the Queue
	for i := 0; i < nbSlots; i++ {
		go func(j int) { availableWorkers <- j }(i)
	}
	i := <-availableWorkers
	t := seed.Sapling()
	t.custodian = "spawner"
	for true {
		log.Printf("%v DEBUG Waiting for an available worker", &t)
		i = <-availableWorkers
		log.Printf("%v DEBUG worker %v waiting on locker\n", &t, i)
		lockerSpawnerSynchro <- i //Signal locker that we are ready
		//to work by sending it a waiting token
		select {
		case i = <-lockerSpawnerSynchro: //Locker gives us our token back: it could
			//not get the locks
			log.Printf("%v DEBUG received token %v, putting it back to the pool\n", &t, i)
			go func(j int) { availableWorkers <- j }(i)
		case t := <-fromLocker:
			t.custodian = "spawner"
			log.Printf("%v DEBUG Assigning to worker %v\n", t, i)
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
	arguments, err := docopt.Parse(usage, nil, true, "Poor Man's Job Queue, v 1.0.0β", false)
	if err != nil {
		log.Fatal(err)
	}
	//log.Println("pmjq started")
	//log.Println(arguments)
	seed := Transition{
		id:              0,
		custodian:       "Seed",
		inputPatterns:   make([]*DirPattern, 0, len(arguments["--input"].([]string))),
		outputTemplates: make([]*DirTemplate, 0, len(arguments["--output"].([]string))),
		cmdTemplate:     template.Must(template.New("Command").Parse(arguments["<cmdtemplate>"].(string))),
	}
	//log.Printf("%v DEBUG Initial seed\n", seed)
	for _, inpattern := range arguments["--input"].([]string) {
		dir, pattern := filepath.Split(inpattern)
		if pattern == "" {
			pattern = ".*" //Unspecified pattern defaults to all files
		}
		seed.inputPatterns = append(seed.inputPatterns,
			&DirPattern{dir, pattern, *regexp.MustCompile(pattern)})
	}
	//log.Printf("%v DEBUG Input patterns\n", seed)
	for i, outtemplate := range arguments["--output"].([]string) {
		dir, tmplt := filepath.Split(outtemplate)
		if tmplt == "" {
			tmplt = "{{.Input 0}}" //Unspecified template defaults to same name as first input file
		}
		seed.outputTemplates = append(seed.outputTemplates,
			&DirTemplate{dir, tmplt,
				*template.Must(template.New(fmt.Sprintf("Output file %v", i)).Parse(tmplt))})
	}
	log.Printf("%v DEBUG Output templates\n", &seed)
	if len(arguments["--error"].([]string)) > 0 {
		for _, errtemplate := range arguments["--error"].([]string) {
			dir, tmplt := filepath.Split(errtemplate)
			if tmplt == "" {
				tmplt = "{{.Input 0}}" //Unspecified template defaults to same name as first input file
			}
			seed.errorTemplates = append(seed.errorTemplates,
				&DirTemplate{dir, tmplt,
					*template.Must(template.New("One of the errors").Parse(tmplt))})
		}
	}
	if arguments["--stderr"] != nil {
		dir, tmplt := filepath.Split(arguments["--stderr"].(string))
		if tmplt == "" {
			tmplt = "{{.Input 0}}" //Unspecified template defaults to same name as first input file
		}
		seed.logTemplate = &DirTemplate{dir, tmplt,
			*template.Must(template.New("The log file").Parse(tmplt))}
	}
	if len(arguments["--input"].([]string)) > 1 {
		seed.invariantTemplate = arguments["--invariant"].(string)
	}
	// cmd_argv, err := shellwords.Parse(arguments["<filter>"].(string))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	fromDirListerToLocker := make(chan *Transition)
	go dirLister(&seed, fromDirListerToLocker, arguments["--quit-when-empty"].(bool))
	fromLockerToSpawner := make(chan *Transition)
	lockerSpawnerSynchro := make(chan int)
	go locker(fromDirListerToLocker, lockerSpawnerSynchro, fromLockerToSpawner)
	go spawner(&seed, lockerSpawnerSynchro, fromLockerToSpawner, 4)

	time.Sleep(3 * time.Second)
	//log.Println("Exiting.")
	c := make(chan int)
	<-c
}
