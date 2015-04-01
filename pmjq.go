package main

// #include <sys/file.h>
import "C" 

import (
	"log"
	"github.com/looplab/fsm"
	"github.com/docopt/docopt-go"
	"time"
	"os"
	"os/exec"
	"io/ioutil"
	"syscall"
)

type State struct {
	main_loop *fsm.FSM
}

func execution(e *fsm.Event, events chan<-string, jobs <-chan*os.File) {
	log.Println("Beginning exec")
	file := <- jobs
	log.Printf("File to exec : %s\n",file.Name())
	cmd := exec.Command("./"+file.Name())
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	go func(cmd *exec.Cmd,f *os.File){
		log.Printf("GOROUTINE THAT WAITS FOR THE JOB %s TO FINISH\n", f.Name())
		err = cmd.Wait()
		log.Printf("Command finished with error: %v... ", err)
		log.Printf("DONE, UNLINKING AND CLOSING FILE %",f.Name())
		syscall.Unlink(f.Name())
		f.Close()
	}(cmd, file)	
	log.Println("Finished launching")
	go func(){events<-"launched"}()
}

func book_keeping(e *fsm.Event, c chan<- string) {
	//log.Println("Beginning book keeping")
	//log.Println("Finished book keeping")
	go func(){c<-"books kept"}()
}

func get_lock(filename string) (*os.File){
	log.Println("\tTrying to get a lock...")
	file,_ := os.Open(filename) //FIXME:Check err
	err := syscall.Flock(int(file.Fd()), C.LOCK_EX + C.LOCK_NB)
	if err != nil { //Unable to obtain lock
		log.Println("\tFile already locked")
		file.Close()
		return nil
	}
	return file
}

func polling(e *fsm.Event, spool_dir string, events chan<- string, jobs chan<- *os.File) {
	log.Println("Beginning polling")
	entries, _ := ioutil.ReadDir(spool_dir) //FIXME:Check err
	for _,file_info := range entries{
		log.Printf("Analyzing file: \t %s\n", file_info.Name())
		//FIXME: check x permission
		file := get_lock(spool_dir+"/"+file_info.Name())
		if file != nil{
			log.Printf("\tJob found: %s\n", file_info.Name())
			go func(){jobs <- file}()
			go func(){events <- "job found"}()
			return
		}
	}
	log.Println("Done polling")
	go func(){events<-"no jobs found"}()
	
}

func waiting(e *fsm.Event, c chan<- string){
	log.Println("Starting to wait")
	time.Sleep(2000 * time.Millisecond)
	log.Println("Waking up")
	go func(){c<-"wake-up"}()
}

func main() {
	/*The main loop is a Finite State Machine.
	FIXME: draw the machine
	*/
	usage := `pmjq
Usage:
	pmjq [-C cpu-limit] [-o archive] <spool-dir>
	pmjq -h | --help
	pmjq --version
	
Options:
	-h --help     Show this screen.
        --version     Show version.
	-C cpu-limit  No jobs are launched if current cpu usage is above limit.
        -o archive    Finished jobs are archived in this directory.
	`

	arguments, err := docopt.Parse(usage, nil, true, "Poor Man's Job Queue, initial dev version.", false)
	if err != nil {
		log.Fatal(err)
	}
	spool_dir := arguments["<spool-dir>"].(string)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	
	//log.Println(arguments)
	event_queue := make(chan string)
	job_queue := make(chan *os.File)
	state := fsm.NewFSM(
	 	"waiting",
	 	fsm.Events{
	 		{Name: "wake-up", Src: []string{"waiting"}, Dst: "book-keeping"},
	 		{Name: "books kept", Src: []string{"book-keeping"}, Dst: "polling"},
	 		{Name: "job found", Src: []string{"polling"}, Dst: "exec-ing"},
	 		{Name: "no jobs found", Src: []string{"polling"}, Dst: "waiting"},
	 		{Name: "launched", Src: []string{"exec-ing"}, Dst: "book-keeping"},
	 	},
	 	fsm.Callbacks{
	 	"book-keeping": func(e *fsm.Event) {book_keeping(e, event_queue)},
	 	"polling":      func(e *fsm.Event) {polling(e, spool_dir, event_queue, job_queue)},
		"waiting":      func(e *fsm.Event) {waiting(e, event_queue)},
		"exec-ing":     func(e *fsm.Event) {execution(e, event_queue, job_queue)},
	 },
	 	)
	//Event loop
	go func(){event_queue <- "wake-up"}()
	cont := true
	for cont {
		event := <-event_queue
		err := state.Event(event)
		if err != nil {
		log.Fatalln(err)
		}
	}
}
