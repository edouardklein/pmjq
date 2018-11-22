(define-module (gnu services toto)
  #:use-module (shepherd service)
  #:use-module (shepherd support)
  #:use-module (oop goops)
  #:export (Arbitrary-service ))

(define Arbitrary-service
  (make <service>
    #:docstring "PMJQ Transition Arbitrary"
    #:provides '(Arbitrary)
    #:requires '()
    #:respawn? #t
    #:start
    (lambda args
      (map (lambda (endpoint)
             (unless (file-exists? endpoint)
               (local-output (string-append "Creating missing endpoint " endpoint))
               (mkdir-p endpoint)))
           '("/tmp/input0" "/tmp/input1" "/tmp/outputA" "/tmp/outputB" "/tmp/error0" "/tmp/error1" "/tmp/log" ))
      ((make-forkexec-constructor
        (list
         "pmjq" "--quit-when-empty" "--input=/tmp/input0/(?P<id>...)_(?P<suffix>.*)\.txt" "--input=/tmp/input1/(?P<prefix>.*)_(?P<id>...)\.txt" "--invariant=$id" "sha256sum" "--output=/tmp/outputA/{{.NamedMatches.id}}_{{.NamedMatches.prefix}}_{{.NamedMatches.suffix}}.txt" "--output=/tmp/outputB/{{.Invariant}}.txt" "--stderr=/tmp/log/{{.Invariant}}.log" "--error=/tmp/error0/{{.Input 0}}" "--error=/tmp/error1/{{.Input 1}}")
        
        #:log-file "/tmp/log/Arbitrary.log")))
    #:stop
    (lambda (running . args)
      ((make-kill-destructor 2) ;SIGINT
       (slot-ref Arbitrary-service 'running))
	    #f)
    #:actions
    (make-actions
     (help
      "Show the help message"
      (lambda _
        (local-output "This service can start, stop and display the status of the Arbitrary pmjq transition."))))))

(register-services Arbitrary-service )
