//pmjq is a daemon that watches a directory and processes any file created therein
package main

// #cgo LDFLAGS: -llockfile
// #include <lockfile.h>
import "C"

import (
	"container/list"
	"github.com/docopt/docopt-go"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"
)

//Concurrency pattern in pmjq: workers talk to each other using channels

//The dir_lister worker feeds the locker files to try to get a lock on.
//It gets those files by listing the input dir again and again, writing
//what it finds on a blocking channel
func dir_lister(input_dirs []string, to_locker chan<- []string) {
	log.Println("dir_lister started")
	time_chan := make(chan int)
	go func() { time_chan <- 0 }()
	for true {
		log.Println("dir_lister: Waiting on time chan")
		_ = <-time_chan
		go func() {
			time.Sleep(3 * time.Second)
			time_chan <- 0
		}()
		//List the input directory(ies)
		dir, err := filepath.Abs(input_dirs[0])
		if err != nil {
			log.Fatal(err)
		}
		entries, err := ioutil.ReadDir(dir)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("dir_lister: listed the input directories", entries)
		//construct an array of arguments
		available_arguments := list.New()
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".lock") { //Lockfiles are not to be processed
				continue
			}
			available_arguments.PushFront([]string{dir + "/" + entry.Name()})
		}
		for args := available_arguments.Front(); args != nil; args = args.Next() {
			//Feed each element to the blocking channel
			log.Println("dir_lister: sending available argument to locker:", args)
			to_locker <- args.Value.([]string)
		}
	}
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
func locker(from_dir_lister <-chan []string, locker_spawner_synchro chan int,
	to_spawner chan<- []string) {
	log.Println("locker started")
	for true {
		log.Println("locker: Waiting on dir_lister to suggest files to try to lock:")
		files := <-from_dir_lister
		log.Println("locker: Got files from dir_lister:", files)
		success := make(chan int)
		release := make(chan int)
		log.Println("locker: Waiting for spawner to be ready to spawn")
		waiting_token := <-locker_spawner_synchro //Will unblock once spawner is ready to spawn
		log.Println("locker: Got token ", waiting_token, " from spawner, acquiring locks")
		for _, file := range files {
			go lock_file(file+".lock", success, release)
		}
		status := 0
		for i, _ := range files {
			status += <-success
			log.Println("locker: After iteration ", i, ", status is ", status)
		}
		if status != 0 { //At least one lock was not acquired
			for i, _ := range files {
				log.Println("locker: Releasing partial lock ", i)
				release <- status
			}
			log.Println("locker: Giving waiting token ", waiting_token, " back to spawner")
			locker_spawner_synchro <- waiting_token
			continue
		}
		//All locks acquired
		log.Println("locker: Sending locked files to spawner:", files)
		to_spawner <- files
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
			break
		}
	}
}

//The spawner worker has a few slots for actual_workers to be launched.
//actual_worker instances are given an input and output channel.
//They read the arguments on the input channel.
//These are given a list of arguments on their input channel as read from locker.
//They send their id on the common output channel when they are done.
func spawner(locker_spawner_synchro chan int, from_locker <-chan []string, nb_slots int) {
	log.Println("spawner started")
	input_channels := make([](chan []string), nb_slots)
	output_channel := make(chan int)
	//Initialize all the channels and the actual_workers that read from them
	for i, _ := range input_channels {
		input_channels[i] = make(chan []string)
		go actual_worker(input_channels[i], i, output_channel)
	}
	i := <-output_channel
	for true {
		log.Println("spawner: actual_woker", i, " is waiting on locker")
		locker_spawner_synchro <- i //Signal locker that we are ready
		//to work by sending it a waiting token
		log.Println("spawner: waiting to hear back from locker")
		select {
		case i = <-locker_spawner_synchro: //Locker gives us our token back: it could
			//not get the locks
			log.Println("spawner: locker is giving our token back: ", i)
		case args := <-from_locker:
			input_channels[i] <- args //Signal this actual_worker to start working
			log.Println("spawner: actual worker ", i,
				"fed, waiting for another worker to be available")
			i = <-output_channel
		}
	}
}

//The actual_workers are tasked with launching and monitoring the data processing tasks.
//They receive their input on the input_channel, and they output their id
//on the output_channel.
func actual_worker(input_channel chan []string, id int, output_channel chan<- int) {
	log.Println("actual_worker ", id, " started")
	output_channel <- id
	for true {
		//Wait for the arguments
		log.Println("actual_worker", id, ": waiting on arguments from spawner")
		args := <-input_channel
		//We launch the command
		log.Println("actual_worker", id, ": got args: ", args)
		for true {
			//TODO: Here actually launch the command

		}
		//Wait for it to finish
		//A write our id to the output channel to say we are done
		output_channel <- id
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	usage := `pmjq.

	Usage: pmjq [options] <input-dir> <filter> <output-dir>
	       pmjq [options] --multi <cmd> <pattern> <indir>...
	       pmjq -h | --help
	       pmjq --version

  Options:
     --help -h   Show this message
     --version   Show version information and exit
     --multi -m  Launch a branching or merging invocation
`
	arguments, err := docopt.Parse(usage, nil, true, "Poor Man's Job Queue, v 1.0.0", false)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("pmjq started")
	log.Println(arguments)
	input_dirs := []string{arguments["<input-dir>"].(string)}
	from_dir_lister_to_locker := make(chan []string)
	go dir_lister(input_dirs, from_dir_lister_to_locker)
	from_locker_to_spawner := make(chan []string)
	locker_spawner_synchro := make(chan int)
	go locker(from_dir_lister_to_locker, locker_spawner_synchro, from_locker_to_spawner)
	go spawner(locker_spawner_synchro, from_locker_to_spawner, 2)

	c := make(chan int)
	<-c
}
