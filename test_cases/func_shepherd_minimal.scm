(define-module (gnu services toto)
  #:use-module (shepherd service)
  #:use-module (shepherd support)
  #:use-module (oop goops)
  #:export (sha256sum-service ))

(define sha256sum-service
  (make <service>
    #:docstring "PMJQ Transition sha256sum"
    #:provides '(sha256sum)
    #:requires '()
    #:respawn? #t
    #:start
    (lambda args
      (map (lambda (endpoint)
             (unless (file-exists? endpoint)
               (local-output (string-append "Creating missing endpoint " endpoint))
               (mkdir-p endpoint)))
           '("/tmp/input" "/tmp/output" ))
      ((make-forkexec-constructor
        (list
         "pmjq" "--input=/tmp/input/" "sha256sum" "--output=/tmp/output/")
        )))
    #:stop
    (lambda (running . args)
      ((make-kill-destructor 2) ;SIGINT
       (slot-ref sha256sum-service 'running))
	    #f)
    #:actions
    (make-actions
     (help
      "Show the help message"
      (lambda _
        (local-output "This service can start, stop and display the status of the sha256sum pmjq transition."))))))

(register-services sha256sum-service )
