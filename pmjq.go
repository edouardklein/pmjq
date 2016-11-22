//pmjq is a daemon that watches a directory and processes any file created therein
package main

// #cgo LDFLAGS: -llockfile
// #include <lockfile.h>
import "C"

import (
	"container/list"
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/mattn/go-shellwords"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type transition struct {
	//id is a unique numerical identifier for a transition (to track its path
	// for debugging purposes)
	id int

	//custodian is the name of the function that owns the structure
	custodian string

	//err is the non-remediable error that derailed this Transition's processing
	err error // FIXME: Use this to process erroneous inputs

	//input_dirs are the directory(ies) in which to look for input files with
	//which to feed the command
	input_dir string

	//output_dirs are the directory(ies) in which the command may write its output
	output_dir string

	//error_dirs are the directory(ies) in which the command will copy error-generating files
	error_dir string

	//input_files are the files from which this particular instance will read
	input_files *list.List

	//output_files are the files in which this particular instance may write
	output_files *list.List

	//error_files are the files in which this particular instance will store
	//error-triggering input files
	error_files *list.List

	//lock_release is a channel on which writing will trigger the release of one
	//locked input or output file (at random depending on the scheduler)
	//to release all locked files, write to it as many times as there are
	//files in the input_files and output_files lists together
	lock_release chan int

	//worker_id is the id number of the worker that will launch the actual command
	worker_id int //FIXME: Use this field

	//cmd_name is the name of the binary that will process the data
	cmd_name string

	//args is the list of arguments to be given to the binary
	args []string

	//cmd is the Cmd structure that controls the actual execution
	cmd *exec.Cmd

	//stdin is the standard input of the process
	stdin io.WriteCloser

	//stdout is the standard output of the process
	stdout io.ReadCloser

	//stderr is the standard error of the process
	stderr io.ReadCloser

	//input_fd is the input file that should be dumped to the stdin of the process
	input_fd io.ReadCloser //FIXME: Use this field

	//output_fd is the output file that should be created from the process' stdout
	output_fd io.WriteCloser //FIXME: Use this field
}

//Pretty print a transition
func log_transition(t transition) {
	//0 Nobody     in/? ?-cmd ???? (????)-> out/? ??????? //Seed
	//1 dir_lister in/f ?-cmd args (????)-> out/f ??????? //with files
	//1 locker     in/f ?-cmd args (????)-> out/f release //with locked files
	//1 worker1    in/f 1-cmd args (1334)-> out/f release //with attributed worker
	//1 worker1    in/f 1-[30]-cmd args (1334)-> out/f release //feeding input
	//1 worker1    in/f 1-cmd args (1334)-[30]-> out/f release //getting output
	in := fmt.Sprintf("%v/?", t.input_dir)
	out := fmt.Sprintf("%v/?", t.output_dir)
	worker_id := "?"
	pid := "?????"
	release := "?"
	if t.input_files != nil {
		in = t.input_files.Front().Value.(string)
		out = t.output_files.Front().Value.(string)
	}
	if t.worker_id != -1 {
		worker_id = fmt.Sprintf("%v", t.worker_id)
	}
	if t.cmd != nil {
		pid = fmt.Sprintf("%v", t.cmd.Process.Pid)
	}
	if t.lock_release != nil {
		release = fmt.Sprintf("%v", t.lock_release)
	}
	log.Printf("%v %v\t%v\t%v-%v %v (%v)-> %v\t%v",
		t.id, t.custodian, in, worker_id, t.cmd_name, t.args, pid, out, release)
}

//Concurrency pattern in pmjq: workers talk to each other using channels

//The dir_lister worker feeds the locker files to try to get a lock on.
//It gets those files by listing the input dir again and again, writing
//what it finds on a blocking channel
//It also provides the files to lock in the output dirs, inferring their names
//from the name of the files in the input dirs
func dir_lister(seed transition, to_locker chan<- transition, quit_empty bool) {
	log.Println("dir_lister started")
	time_chan := make(chan int)
	go func() { time_chan <- 0 }()
	last_id := seed.id
	for true {
		log.Println("dir_lister: Waiting on time chan")
		_ = <-time_chan
		go func() {
			time.Sleep(3 * time.Second)
			time_chan <- 0
		}()
		//List the input directory(ies)
		entries, err := ioutil.ReadDir(seed.input_dir)
		if err != nil {
			log.Fatal(err)
		}
		if quit_empty && len(entries) == 0 {
			log.Println("Nothing left to do, exiting")
			os.Exit(0)
		}
		log.Println("dir_lister: listing the input directory")
		//construct a list of transitions
		transitions := list.New()
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".lock") { //Lockfiles are not to be processed
				continue
			}
			t := seed
			last_id += 1
			t.id = last_id
			t.custodian = "dir_lister"
			t.input_files = list.New()
			t.input_files.PushFront(t.input_dir + "/" + entry.Name())
			t.output_files = list.New()
			t.output_files.PushFront(t.output_dir + "/" + entry.Name())
			if t.error_dir != "" {
				t.error_files = list.New()
				t.error_files.PushFront(t.error_dir + "/" + entry.Name())
			}
			log_transition(t)
			transitions.PushFront(t)
		}
		for args := transitions.Front(); args != nil; args = args.Next() {
			//Feed each element to the blocking channel
			log.Println("dir_lister: sending available argument to locker:")
			log_transition(args.Value.(transition))
			to_locker <- args.Value.(transition)
		}
	}
}

//This function is the abort function for the locker, when something went
//during lock aquisition
func lock_abort(release_chan chan int, nb_locks int, waiting_token int,
	locker_spawner_synchro chan int) {
	for i := 0; i < nb_locks; i += 1 {
		log.Println("locker_abort: Releasing partial lock ", i)
		release_chan <- 1
	}
	log.Println("locker_abort: Giving waiting token ", waiting_token, " back to spawner")
	locker_spawner_synchro <- waiting_token
}

//The locker worker tries to get a lock on the arguments provided to it by dir_lister.
//It waits for the spawner to have an available slot before it tries to get a lock
//(this avoid having a lock on files and waiting, while some other machine could
//process them but does not have the lock)
//The spawner tells the locker that a slot is available by writing on channel
//locker_spawner_synchro.
//If it can not get a lock on all the files, the locker signals so by sending a token
//back to the spawner through locker_spawner synchro.
//Once all arguments to a call have been locked, it passes them on to the spawner
//via the to_spawner channel.
func locker(from_dir_lister <-chan transition, locker_spawner_synchro chan int,
	to_spawner chan<- transition) {
	log.Println("locker started")
	for true {
		log.Println("locker: Waiting on dir_lister to suggest files to try to lock:")
		t := <-from_dir_lister
		t.custodian = "locker"
		success := make(chan int)
		t.lock_release = make(chan int)
		files := list.New()
		files.PushFrontList(t.input_files)
		files.PushFrontList(t.output_files)
		log_transition(t)
		log.Println("locker: Waiting for spawner to be ready to spawn")
		waiting_token := <-locker_spawner_synchro //Will unblock once spawner is ready to spawn
		log.Println("locker: Got token ", waiting_token, " from spawner, acquiring locks")
		for file := files.Front(); file != nil; file = file.Next() {
			go lock_file(file.Value.(string)+".lock", success, t.lock_release)
		}
		status := 0
		for i := 0; i < files.Len(); i += 1 {
			status += <-success
			log.Println("locker: After iteration ", i, ", status is ", status)
		}
		if status != 0 { //At least one lock was not acquired
			lock_abort(t.lock_release, files.Len(), waiting_token,
				locker_spawner_synchro)
			continue
		}
		//All locks acquired
		status = 0
		for file := t.input_files.Front(); file != nil; file = file.Next() {
			if _, err := os.Stat(file.Value.(string)); os.IsNotExist(err) {
				//If file does not exist
				status = -1
				break
			}
		}
		if status != 0 { //At least one file no longer exists
			lock_abort(t.lock_release, files.Len(), waiting_token,
				locker_spawner_synchro)
			continue
		}
		//All files exist
		log.Println("locker: Sending locked files to spawner:")
		log_transition(t)
		to_spawner <- t
	}
}

//The lock_file function creates a lock on the given file.
//It defers the removal of the lock.
//It refreshes the lock every minute (see the man page for lockfile_create).
//It exits only when something is written to the release channel.
//It writes its status (0:success, !=0: failure) on the success channel.
func lock_file(fname string, success chan<- int, release <-chan int) {
	log.Println("lock_file acquiring ", fname)
	err_int := int(C.lockfile_create(C.CString(fname), 0, 0))
	if err_int != 0 {
		log.Println("lock_file: Could not get a lock on ", fname, "error nb ", err_int)
		success <- err_int
		i := <-release
		log.Println("lock_file", fname, " exiting", i)
		return
	}
	success <- 0
	defer func() {
		log.Println("lock_file defer release lock on ", fname)
		err_int = int(C.lockfile_remove(C.CString(fname)))
		log.Println("lock_file lock released on ", fname, ": ", err_int)
	}()
	time_chan := make(chan int)
	go func() { time_chan <- 0 }()
	for true {
		select {
		case _ = <-time_chan:
			log.Println("lock_file refreshing lock ", fname)
			C.lockfile_touch(C.CString(fname))
			go func() {
				time.Sleep(60 * time.Second)
				time_chan <- 0
			}()
		case i := <-release:
			log.Println("lock_file ", fname, "exiting ", i)
			return
		}
	}
}

// Read from src in buckets of 4096 and dumps them in dst
// It logs everything in the process, appending logid before
func get_bucket_dumper(logid string, src io.ReadCloser, dst io.WriteCloser) (func(), chan error) {
	c := make(chan error)
	return func() {
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
			log.Println(logid, " read ", data)
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
	}, c
}

//The actual_workers are tasked with launching and monitoring the data processing tasks.
//They receive their input on the input_channel, and they output their id
//on the output_channel.
func get_actual_worker(seed transition) func(chan transition, int, chan<- int) {
	return func(input_channel chan transition, id int, output_channel chan<- int) {
		log.Println("actual_worker ", id, " started")
		for true {
			//Launch the process (done first because it can take some time)
			log.Println("actual_worker", id, ": Launching a new process")
			cmd := exec.Command(seed.cmd_name, seed.args...)
			stdin, err := cmd.StdinPipe()
			if err != nil {
				log.Fatal(err)
			}
			defer stdin.Close()
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				log.Fatal(err)
			}
			defer stdout.Close()
			stderr, err := cmd.StderrPipe()
			if err != nil {
				log.Fatal(err)
			}
			defer stderr.Close()
			err = cmd.Start()
			if err != nil {
				log.Fatal(err)
			}
			//Wait for the data
			log.Println("actual_worker", id, ": waiting on data from spawner")
			output_channel <- id
			t := <-input_channel
			t.custodian = fmt.Sprintf("actual_worker %v", id)
			t.worker_id = id
			t.cmd = cmd
			t.stdin = stdin
			t.stdout = stdout
			t.stderr = stderr
			log.Println("actual_worker", id, ": got args:")
			log_transition(t)
			//Launch a worker that reads from disk and writes to the stdin of the command
			//Wrapping it in an anonymous func so that Close() is called as soon
			//as we are finished with the FDs
			// http://grokbase.com/t/gg/golang-nuts/134883hv3h/go-nuts-io-closer-and-closing-previously-closed-object
			if err := func() error {
				t.input_fd, err = os.Open(t.input_files.Front().Value.(string))
				if err != nil {
					log.Fatal(err)
				}
				defer t.input_fd.Close()
				disk_to_stdin, d2schan := get_bucket_dumper(t.custodian+" disk->stdin ",
					t.input_fd, t.stdin)
				go disk_to_stdin()
				//Launch a worker that reads from the command and writes to disk
				t.output_fd, err = os.Create(t.output_files.Front().Value.(string))
				if err != nil {
					log.Fatal(err)
				}
				defer t.output_fd.Close()
				stdout_to_disk, s2dchan := get_bucket_dumper(t.custodian+" stdout->disk ",
					t.stdout, t.output_fd)
				go stdout_to_disk()
				//FIXME: Launch a worker that reads from the command's stderr and logs it
				//Wait for it to finish
				log.Println("actual_worker", id, "waiting for job to finish")
				log_transition(t)
				<-d2schan
				<-s2dchan
				return cmd.Wait()
			}(); err != nil {
				if t.error_dir == "" {
					log.Fatal(err)
				}
				//Move the input files to the error_dir
				err = os.Rename(t.input_files.Front().Value.(string),
					t.error_files.Front().Value.(string))
				if err != nil {
					log.Fatal(err)
				}
				//Remove the (probably incomplete) output file
				os.Remove(t.output_files.Front().Value.(string))
			} else {
				//Remove the file from the input folder
				err = os.Remove(t.input_files.Front().Value.(string))
				if err != nil {
					log.Fatal(err)
				}
			}
			//Release the file locks
			t.lock_release <- 0
			t.lock_release <- 0
		}
	}
}

//The spawner worker has a few slots for actual_workers to be launched.
//actual_worker instances are given an input and output channel.
//They read the arguments on the input channel.
//These are given a list of arguments on their input channel as read from locker.
//They send their id on the common output channel when they are done.
func spawner(seed transition,
	locker_spawner_synchro chan int, from_locker <-chan transition, nb_slots int) {
	log.Println("spawner started")
	input_channels := make([](chan transition), nb_slots)
	output_channel := make(chan int)
	//Initialize all the channels and the actual_workers that read from them
	for i, _ := range input_channels {
		input_channels[i] = make(chan transition)
		actual_worker := get_actual_worker(seed)
		go actual_worker(input_channels[i], i, output_channel)
	}
	i := <-output_channel
	for true {
		log.Println("spawner: actual_woker", i, " is waiting on locker")
		locker_spawner_synchro <- i //Signal locker that we are ready
		//to work by sending it a waiting token
		select {
		case i = <-locker_spawner_synchro: //Locker gives us our token back: it could
			//not get the locks
			log.Println("spawner: locker is giving our token back: ", i)
		case t := <-from_locker:
			t.custodian = "spawner"
			input_channels[i] <- t //Signal this actual_worker to start working
			log.Println("spawner: actual worker ", i, "fed with")
			log_transition(t)
			log.Println("spawner: waiting for another worker to be available")
			i = <-output_channel
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	usage := `pmjq.

	Usage: pmjq [--quit-when-empty] [--error-dir=<error-dir>] <input-dir> <filter> <output-dir>
	       pmjq -h | --help
	       pmjq --version

  Options:
     --help -h                Show this message
     --version                Show version information and exit
     --quit-when-empty        Exit with 0 status when the input dir is empty
     --error-dir=<error-dir>  If specified, dont crash on error but move incriminated file to this dir
`
	arguments, err := docopt.Parse(usage, nil, true, "Poor Man's Job Queue, v 1.0.0", false)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("pmjq started")
	log.Println(arguments)
	input_dir, err := filepath.Abs(arguments["<input-dir>"].(string))
	if err != nil {
		log.Fatal(err)
	}
	output_dir, err := filepath.Abs(arguments["<output-dir>"].(string))
	if err != nil {
		log.Fatal(err)
	}
	var error_dir string
	if arguments["--error-dir"] != nil {
		error_dir, err = filepath.Abs(arguments["--error-dir"].(string))
		if err != nil {
			log.Fatal(err)
		}
	}
	cmd_argv, err := shellwords.Parse(arguments["<filter>"].(string))
	if err != nil {
		log.Fatal(err)
	}
	seed := transition{
		id:         0,
		custodian:  "Nobody",
		input_dir:  input_dir,
		output_dir: output_dir,
		error_dir:  error_dir,
		worker_id:  -1,
		cmd_name:   cmd_argv[0],
		args:       cmd_argv[1:],
	}
	from_dir_lister_to_locker := make(chan transition)
	go dir_lister(seed, from_dir_lister_to_locker, arguments["--quit-when-empty"].(bool))
	from_locker_to_spawner := make(chan transition)
	locker_spawner_synchro := make(chan int)
	go locker(from_dir_lister_to_locker, locker_spawner_synchro, from_locker_to_spawner)
	go spawner(seed, locker_spawner_synchro, from_locker_to_spawner, 15)

	c := make(chan int)
	<-c
}
