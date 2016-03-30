/*
PMJQ is a tool to tool

Heading1

pargraphparagtrapg

paragaph ?

Paragraph.

Heading2

paragraph
*/
package main

// #include <sys/file.h>
import "C"

import (
	"github.com/docopt/docopt-go"
	"github.com/looplab/fsm"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func execing(e *fsm.Event, archive_dir string, error_dir string, events chan<- string, jobs <-chan *os.File) {
	file := <-jobs
	cmd := exec.Command("./" + file.Name())
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	go func(cmd *exec.Cmd, f *os.File) {
		log.Printf("INFO: action=exec file=%s\n", f.Name())
		err = cmd.Wait()
		if err == nil {
			if archive_dir != "" {
				log.Printf("INFO: action=archive file=%s\n", f.Name())
				err := exec.Command("cp", f.Name(),
					archive_dir+"/"+time.Now().Local().Format("20060102-15:04:05-")+path.Base(f.Name())).Run()
				if err != nil {
					log.Fatal(err)
				}
			}
		} else { // Command said something on stderr
			log.Printf("WARNING: event=cmd_error file=%s error=%v", f.Name(), err)
			if error_dir != "" {
				log.Printf("WARNING: action=error_archive file=%s\n", f.Name())
				err := exec.Command("cp", f.Name(),
					error_dir+"/"+time.Now().Local().Format("20060102-15:04:05-")+path.Base(f.Name())).Run()
				if err != nil {
					log.Fatal(err)
				}
			}
		}
		log.Printf("INFO: event=exec_end file=%s\n", f.Name())
		syscall.Unlink(f.Name())
		f.Close()
	}(cmd, file)
	go func() { events <- "launched" }()
}

func book_keeping(e *fsm.Event, cpu_check float32, c chan<- string) {
	// Mise à jour du load average
	// Lancement d'une nouvelle instance
	// Checking on existing processes
	// Polling or waiting
	log.Println("DEBUG: action=book_keeping")
	if cpu_check != -1 {
		if Load_average > cpu_check { //Not launching a new job now
			log.Println("WARNING: event=high_load message=Load average too high, going back to sleep without launching a new job")
			go func() { c <- "books say sleep" }()
			return
		}
	}
	log.Println("DEBUG: event=books_kept")
	go func() { c <- "books kept" }()
}

func get_lock(filename string) *os.File {
	log.Printf("INFO: action=lock file=%s\n", filename)
	file, _ := os.Open(filename) //FIXME:Check err
	err := syscall.Flock(int(file.Fd()), C.LOCK_EX+C.LOCK_NB)
	if err != nil { //Unable to obtain lock
		log.Printf("INFO: event=already_locked file=%s", filename)
		file.Close()
		return nil
	}
	log.Printf("INFO: event=locked file=%s", filename)
	return file
}

func polling(e *fsm.Event, spool_dir string, events chan<- string, jobs chan<- *os.File) {
	log.Println("DEBUG: action=polling")
	entries, _ := ioutil.ReadDir(spool_dir) //FIXME:Check err
	for _, file_info := range entries {
		log.Printf("INFO: event=file_found file=%s\n", file_info.Name())
		//FIXME: check x permission
		file := get_lock(spool_dir + "/" + file_info.Name())
		if file != nil {
			log.Printf("INFO: event=job_found file=%s\n", file_info.Name())
			go func() { jobs <- file }()
			go func() { events <- "job found" }()
			return
		}
	}
	log.Println("DEBUG: event=done_polling")
	go func() { events <- "no jobs found" }()

}

func waiting(e *fsm.Event, c chan<- string) {
	log.Println("DEBUG: action=waiting")
	time.Sleep(2000 * time.Millisecond)
	log.Println("DEBUG: event=waking_up")
	go func() { c <- "wake-up" }()
}

func load_average() float32 {
	uptime_string, err := exec.Command("uptime").Output()
	if err != nil {
		log.Fatal(err)
	}
	uptime_array := strings.Split(string(uptime_string), " ")
	la_float, err := strconv.ParseFloat(uptime_array[len(uptime_array)-3], 32) //Load average over the last minute,
	//see man page for uptime(2)
	if err != nil {
		log.Fatal(err)
	}
	return float32(la_float)
}

/*GLOBAL VARIABLE*/
var Load_average = float32(-1) //Load average of the system over the last minute , updated by a goroutine, read by the book-keeper if -C option is passed

// PMJQ is a bla bla
func main() {
	/*The main loop is a Finite State Machine.
	FIXME: draw the machine
	*/
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	usage := `pmjq
Usage:
	pmjq [options] <input-dir> <filter> <output-dir>
  pmjq [options] --inputs <pattern> <indir>... --cmd <cmd>
	pmjq -h | --help
	pmjq --version

Options:
	-h --help       Show this screen.
  --version       Show version.
	-C <cpu-limit>  No jobs are launched if current load is above limit.
  --max-load <max-load> <max-cmd>  Command to launch if load goes above the upper limit.
  --min-load <min-load> <min-cmd>  Command to launch if load stays below lower limit.
	`
	arguments, err := docopt.Parse(usage, nil, true, "Poor Man's Job Queue, initial dev version.", false)
	//load_average()
	if err != nil {
		log.Fatal(err)
	}
	spool_dir := arguments["<spool-dir>"].(string)
	archive_dir := ""
	if arguments["-o"] != nil {
		archive_dir = arguments["-o"].(string)
	}
	error_dir := ""
	if arguments["-e"] != nil {
		error_dir = arguments["-e"].(string)
	}
	cpu_check := float32(-1)
	if arguments["-C"] != nil {
		f, err := strconv.ParseFloat(arguments["-C"].(string), 32)
		cpu_check = float32(f)
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			for true {
				Load_average = load_average()
				log.Printf("INFO: load_average=%f\n", Load_average)
				time.Sleep(5000 * time.Millisecond) //FIXME put it back to 60000
			}
		}()
	}

	//log.Println(arguments)
	event_queue := make(chan string)
	job_queue := make(chan *os.File)
	state := fsm.NewFSM(
		"waiting",
		fsm.Events{
			{Src: []string{"waiting"}, Dst: "book-keeping", Name: "wake-up"},
			{Src: []string{"book-keeping"}, Dst: "polling", Name: "books kept"},
			{Src: []string{"book-keeping"}, Dst: "waiting", Name: "books say sleep"},
			{Src: []string{"polling"}, Dst: "execing", Name: "job found"},
			{Src: []string{"polling"}, Dst: "waiting", Name: "no jobs found"},
			{Src: []string{"execing"}, Dst: "book-keeping", Name: "launched"},
		},
		fsm.Callbacks{
			"book-keeping": func(e *fsm.Event) { book_keeping(e, cpu_check, event_queue) },
			"polling":      func(e *fsm.Event) { polling(e, spool_dir, event_queue, job_queue) },
			"waiting":      func(e *fsm.Event) { waiting(e, event_queue) },
			"execing":      func(e *fsm.Event) { execing(e, archive_dir, error_dir, event_queue, job_queue) },
		},
	)
	//Event loop
	go func() { event_queue <- "wake-up" }()
	cont := true
	for cont {
		event := <-event_queue
		err := state.Event(event)
		if err != nil {
			log.Fatalln(err)
		}
	}
}
