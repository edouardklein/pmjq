/*
Go code for the PMJQ daemon.

PMJQ comes with a full-fledged design and monitoring suite, but only
this executable is needed on the target machine.
*/
package main

// #include <fcntl.h>
// int fcntl_wrapper(int fd, int cmd, struct flock* fl){
// //Variadic functions like fcntl can't be called from go, hence the wrapper
// return fcntl(fd, cmd, fl);
// }
import "C"

import (
	"container/heap"
	"github.com/docopt/docopt-go"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	// 	"strconv"
	// 	"strings"
	// 	"syscall"
	// "time"
)

//*****************
// Thread functions
//*****************
// Those functions are called as goroutines,
// working together to make the whole program work
// see the coordinator() for the entry point

func coordinator(input_dir string, worker func(string)) {
	// The coordinator maintains a todo list (a priority queue)
	// It gives elements of the priority queue to workers that
	// will process them.
	// When the queue is empty, it triggers a complete scan of the input folder(s),
	// looking for things to do.
	// TODO: Only launch workers if the load average is below a limit
	// TODO: When a worker reports an error, move the offending files out of the way
	// TODO: Somehow keep an eye on active workers, killing those that take too long
	todo := make(PriorityQueue, 0)
	heap.Init(&todo)
	for true {
		if todo.Len() == 0 {
			//TODO: Limit the busy polling by waiting on a timed channel (like <-c; scan; go func sleep toto -> c)
			scanner(input_dir, &todo)
		} else {
			item := heap.Pop(&todo).(*Item)
			go worker(item.value)
		}
	}
}

func scanner(input_dir string, pq *PriorityQueue) {
	// The scanner list the input dirs, looking for files to process.
	// It add those files, along with the time at which they were created,
	// to the priority queue
	log.Println("DEBUG: actor=scanner")
	entries, _ := ioutil.ReadDir(input_dir) //FIXME:Check err
	for _, file_info := range entries {
		log.Printf("DEBUG: actor=scanner event=file_found file=%s\n", file_info.Name())
		lock, err := is_locked(input_dir + "/" + file_info.Name())
		if err != nil {
			log.Printf("WARNING: actor=scanner event=couldnt_get_lock_status file=%s\n", file_info.Name())
		} else if lock {
			log.Printf("DEBUG: actor=scanner event=locked_file file=%s\n", file_info.Name())
		} else {
			log.Printf("DEBUG: actor=scanner event=unlocked_file file=%s\n", file_info.Name())
			item := &Item{
				value:    file_info.Name(),
				priority: 1, //FIXME: Put the date and sort with minimum first
			}
			heap.Push(pq, item)
		}
	}
}

func get_filter_worker(strcmd string, outdir string) func(string) {
	cmd := exec.Command(strcmd) //FIXME: Seperate command and args
	return func(infname string) {
		//The worker must acquire two locks :
		// - one on the input file
		// - one on the output file
		// It will compete with other workers with the same input and output
		// (on other machines, it makes no
		// sense to run the same worker multiple times concurrrently
		// on one machine, except for testing)
		// for the input lock.
		// It will compete with other workers (possibly on the same machine)
		// whose inputs include its output
		// for the output lock.

		// Acquire the input lock
		infile, err := get_locked_fd(infname)
		if err != nil {
			log.Printf("WARNING: actor=worker event=couldnt_get_lock_on_infile file=%s error=%s\n", infname, err)
			return
		}
		defer infile.Close() // Close() releases the lock
		log.Printf("DEBUG: actor=worker event=got_locked_infile_fd file=%s\n", infname)

		//Read input file
		indata, err := ioutil.ReadFile(infname)
		if err != nil {
			log.Printf("ERRROR: actor=worker event=couldnt_read_infile file=%s\n", infname)
			return
		}
		if len(indata) == 0 {
			log.Printf("ERRROR: actor=worker event=infile_empty file=%s\n", infname)
		}
		log.Printf("DEBUG: actor=worker event=read_input_file file=%s\n", infname)

		// Acquire the output lock
		outfname := outdir + "/" + path.Base(infname)
		outfile, err := create_locked_fd(outfname)
		if err != nil {
			log.Printf("ERROR: actor=worker event=couldnt_create_and_lock_outfile file=%s\n", infname)
			return
		}
		defer outfile.Close() // Close() releases the lock
		log.Printf("DEBUG: actor=worker event=got_locked_outfile_fd file=%s\n", infname)

		//Get stdin and stdout fd
		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Printf("ERROR: actor=worker event=couldnt_get_stdin file=%s\n", infname)
			return
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("ERROR: actor=worker event=couldnt_get_stdout file=%s\n", infname)
			return
		}
		log.Printf("DEBUG: actor=worker event=got_cmd_stdio file=%s\n", infname)
		//Launch the command
		if err := cmd.Start(); err != nil {
			log.Printf("ERRROR: actor=worker event=command_failed_to_launch file=%s\n", infname)
			return
		}
		log.Printf("DEBUG: actor=worker event=cmd_launched file=%s\n", infname)
		//Dump the infile's contents in the command's stdin
		_, err = stdin.Write(indata)
		err2 := stdin.Close()
		if err != nil || err2 != nil {
			log.Printf("ERRROR: actor=worker event=command_failed_when_reading_input file=%s\n", infname)
			return
		}
		log.Printf("DEBUG: actor=worker event=cmd_working file=%s\n", infname)
		//Wait for the command to finish
		if err := cmd.Wait(); err != nil {
			log.Printf("ERRROR: actor=worker event=command_exited_badly file=%s\n", infname)
			return
		}
		log.Printf("DEBUG: actor=worker event=cmd_finished file=%s\n", infname)
		//Dump the command's stdout in the outfile
		outdata, err := ioutil.ReadAll(stdout)
		if err != nil {
			log.Printf("ERRROR: actor=worker event=failed_to_read_command_output file=%s\n", infname)
			return
		}
		log.Printf("DEBUG: actor=worker event=read_output file=%s\n", infname)
		if _, err := outfile.Write(outdata); err != nil {
			log.Printf("ERRROR: actor=worker event=failed_to_write_in_output_file file=%s\n", infname)
			return
		}
		log.Printf("DEBUG: actor=worker event=wrote_output file=%s\n", infname)
	}
}

//****************
// Lock functions
//****************
// These functions wrap calls to fcntl.
// They let us work with file locks

func is_locked(fname string) (ans bool, err error) {
	// Check if exists and all that jazz
	_, err = os.Stat(fname)
	if err != nil {
		log.Printf("ERRROR: func=is_locked event=file_unstatable file=%s\n", fname)
		return false, err
	}
	// Check if locked
	fd, err := os.Open(fname)
	if err != nil {
		log.Printf("ERRROR: func=is_locked event=file_unopenable file=%s\n", fname)
		return false, err
	}
	fl := C.struct_flock{l_type: C.F_WRLCK} // ReadWrite lock
	_, err = C.fcntl_wrapper(C.int(fd.Fd()), C.F_GETLK, &fl)
	if err != nil {
		log.Printf("ERRROR: func=is_locked event=could_not_get_locked_status file=%s\n", fname)
		return
	}
	if fl.l_type != C.F_UNLCK {
		return true, nil
	}
	return false, nil
}

func create_locked_fd(fname string) (fd *os.File, err error) {
	// Check if exists
	if _, err := os.Stat(fname); os.IsNotExist(err) == false {
		log.Printf("ERRROR: func=create_locked_fd event=file_exists_or_is_unstatable file=%s\n", fname)
		return nil, err
	}
	// Create
	fd, err = os.Open(fname)
	if err != nil {
		log.Printf("ERRROR: func=create_locked_fd  event=file_unopenable file=%s\n", fname)
		return nil, err
	}
	// Acquire lock
	fl := C.struct_flock{l_type: C.F_WRLCK} // ReadWrite lock
	_, err = C.fcntl_wrapper(C.int(fd.Fd()), C.F_SETLK, &fl)
	if err != nil {
		log.Printf("ERRROR: func=create_locked_fd event=couldnt_acquire_lock file=%s\n", fname)
		fd.Close()
		return nil, err
	}
	return fd, nil
}

func get_locked_fd(fname string) (fd *os.File, err error) {
	// Check if exists
	// Check if locked
	// Acquire lock
	// Open
	return nil, nil
}

// func waiting(e *fsm.Event, c chan<- string) {
// 	log.Println("DEBUG: action=waiting")
// 	time.Sleep(2000 * time.Millisecond)
// 	log.Println("DEBUG: event=waking_up")
// 	go func() { c <- "wake-up" }()
// }

// func book_keeping(e *fsm.Event, c chan<- string) {
// 	// Prise de la dernière valeur du load average
// 	// Lancement d'une nouvelle instance
// 	// Checking on existing processes
// 	// Polling or waiting
// 	log.Println("DEBUG: action=book_keeping")
// 	log.Println("DEBUG: event=books_kept")
// 	go func() { c <- "books kept" }()
// }

// func polling(e *fsm.Event, events chan<- string) {
// 	log.Println("DEBUG: action=polling")
// 	entries, _ := ioutil.ReadDir(spool_dir) //FIXME:Check err
// 	for _, file_info := range entries {
// 		log.Printf("INFO: event=file_found file=%s\n", file_info.Name())
// 		//FIXME: check x permission
// 		file := get_lock(spool_dir + "/" + file_info.Name())
// 		if file != nil {
// 			log.Printf("INFO: event=job_found file=%s\n", file_info.Name())
// 			go func() { jobs <- file }()
// 			go func() { events <- "job found" }()
// 			return
// 		}
// 	}
// 	log.Println("DEBUG: event=done_polling")
// 	go func() { events <- "no jobs found" }()

// }
// func execing(e *fsm.Event, archive_dir string, error_dir string, events chan<- string, jobs <-chan *os.File) {
// 	file := <-jobs
// 	cmd := exec.Command("./" + file.Name())
// 	err := cmd.Start()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	go func(cmd *exec.Cmd, f *os.File) {
// 		log.Printf("INFO: action=exec file=%s\n", f.Name())
// 		err = cmd.Wait()
// 		if err == nil {
// 			if archive_dir != "" {
// 				log.Printf("INFO: action=archive file=%s\n", f.Name())
// 				err := exec.Command("cp", f.Name(),
// 					archive_dir+"/"+time.Now().Local().Format("20060102-15:04:05-")+path.Base(f.Name())).Run()
// 				if err != nil {
// 					log.Fatal(err)
// 				}
// 			}
// 		} else { // Command said something on stderr
// 			log.Printf("WARNING: event=cmd_error file=%s error=%v", f.Name(), err)
// 			if error_dir != "" {
// 				log.Printf("WARNING: action=error_archive file=%s\n", f.Name())
// 				err := exec.Command("cp", f.Name(),
// 					error_dir+"/"+time.Now().Local().Format("20060102-15:04:05-")+path.Base(f.Name())).Run()
// 				if err != nil {
// 					log.Fatal(err)
// 				}
// 			}
// 		}
// 		log.Printf("INFO: event=exec_end file=%s\n", f.Name())
// 		syscall.Unlink(f.Name())
// 		f.Close()
// 	}(cmd, file)
// 	go func() { events <- "launched" }()
// }

// func get_lock(filename string) *os.File {
// 	log.Printf("INFO: action=lock file=%s\n", filename)
// 	file, _ := os.Open(filename) //FIXME:Check err
// 	err := syscall.Flock(int(file.Fd()), C.LOCK_EX+C.LOCK_NB)
// 	if err != nil { //Unable to obtain lock
// 		log.Printf("INFO: event=already_locked file=%s", filename)
// 		file.Close()
// 		return nil
// 	}
// 	log.Printf("INFO: event=locked file=%s", filename)
// 	return file
// }

// func load_average() float32 {
// 	uptime_string, err := exec.Command("uptime").Output()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	uptime_array := strings.Split(string(uptime_string), " ")
// 	la_float, err := strconv.ParseFloat(uptime_array[len(uptime_array)-3], 32) //Load average over the last minute,
// 	//see man page for uptime(2)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	return float32(la_float)
// }

// /*GLOBAL VARIABLE*/
// var Load_average = float32(-1) //Load average of the system over the last minute , updated by a goroutine, read by the book-keeper if -C option is passed

func main() {
	// The main loop is a Finite State Machine.
	// Please see the Python documentation for more details, godoc is too bare for my taste
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
	arguments, err := docopt.Parse(usage, nil, true, "Poor Man's Job Queue, initial dev version.", false)
	if err != nil {
		log.Fatal(err)
	}
	if arguments["multi"] == true {
		//TODO Parse
	}
	// event_queue := make(chan string)
	// state := fsm.NewFSM(
	// 	"waiting",
	// 	fsm.Events{
	// 		{Src: []string{"waiting"}, Dst: "book-keeping", Name: "wake-up"},
	// 		{Src: []string{"book-keeping"}, Dst: "polling", Name: "books kept"},
	// 		{Src: []string{"book-keeping"}, Dst: "waiting", Name: "books say sleep"},
	// 		{Src: []string{"polling"}, Dst: "execing", Name: "job found"},
	// 		{Src: []string{"polling"}, Dst: "waiting", Name: "no jobs found"},
	// 		{Src: []string{"execing"}, Dst: "book-keeping", Name: "launched"},
	// 	},
	// 	fsm.Callbacks{
	// 		"book-keeping": func(e *fsm.Event) { book_keeping(e, event_queue) },
	// 		"polling":      func(e *fsm.Event) { polling(e, event_queue) },
	// 		"waiting":      func(e *fsm.Event) { waiting(e, event_queue) },
	// 		"execing":      func(e *fsm.Event) { execing(e, event_queue) },
	// 	},
	// )
	// //Event loop
	// go func() { event_queue <- "wake-up" }()
	// cont := true
	// for cont {
	// 	event := <-event_queue
	// 	err := state.Event(event)
	// 	if err != nil {
	// 		log.Fatalln(err)
	// 	}
	// }
}
