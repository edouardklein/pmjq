/*
Go code for the PMJQ daemon.

PMJQ comes with a full-fledged design and monitoring suite, but only
this executable is needed on the target machine.
*/
package main

// // #include <sys/file.h>
// import "C"

import (
	"container/heap"
	"github.com/docopt/docopt-go"
	"io/ioutil"
	"log"
	// 	"os"
	"os/exec"
	// 	"path"
	// 	"strconv"
	// 	"strings"
	// 	"syscall"
	// "time"
)

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
		if is_locked(input_dir + "/" + file_info.Name()) {
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

func get_filter_worker(strcmd string) func(string) {
	cmd := exec.Command(strcmd) //FIXME: Seperate command and args
	return func(filename string) {
		file, err := get_lock(filename)
		if err != nil {
			log.Printf("WARNING: actor=worker event=couldnt_get_lock file=%s\n", filename)
			return
		}
		log.Printf("DEBUG: actor=worker event=got_lock file=%s\n", filename)
		//Open input file
		//Open output file
		//Get stdin and stdout fd
		//Launch the command
		//Dump infile contents to stdin while dumping stdout to outfile
		//Or just do it in one go
	}
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
