(import shlex)

(defn pmjq-command [transition-sexpr]
  "Return the command to launch in a shell to activate the given transition"
  (defn transition [&kwargs kwargs]
    (+ "pmjq "
       (if (in "log" kwargs)
         (+ "--log-dir=" (get kwargs "log") " ")
         "")
       (if (in "error" kwargs)
         (+ "--error-dir=" (get kwargs "error") " ")
         "")
       (if (and (in "quit_empty" kwargs) (get kwargs "quit_empty"))
         "--quit-when-empty "
         "")
       (get (get kwargs "inputs") 0) " "
       (get kwargs "cmd") " "
       (get (get kwargs "outputs") 0) " "
       (if (in "pmjq_log" kwargs)
         (+ "2>" (get kwargs "pmjq_log"))
         "")))
  (eval transition-sexpr))

(def *start-tmux-session-template* "
if tmux list-sessions | grep {id}; then
  tmux kill-session -t {id}
fi
tmux new-session -d -s {id} bash
for couple in $(env); do
  tmux setenv -t {id} $(echo $couple | tr '=' ' ')
done
#Creating a panel and closing the old one so that the new one gets the environment we want
tmux split-window -t {id} bash
tmux kill-pane -t 1 -a
")

(defn pmjq-tmux-supervision [transition-sexpr &optional [htop True]]
  "Return the command to launch in a shell to supervize the execution
of the given transition

The targeted layout is:

  +-------------------------+
  | (htop)                  |
  +-------------+-----------+
  | inputs      | outputs   |
  +-------------+-----------+
  | (logs)      | (errors)  |
  +-------------+-----------+
  | (pmjq_logs)             |
  +-------------------------+
Panes between () are optional and will not be created if there is no need for them."
  (defn transition [&kwargs kwargs]
    (setv id (get kwargs "id"))
    (setv log (if (in "log" kwargs) (get kwargs "log") False))
    (setv error (if (in "error" kwargs) (get kwargs "error") False))
    (setv pmjq_log (if (in "pmjq_log" kwargs) (get kwargs "pmjq_log") False))
    (setv input (get  (get kwargs "inputs") 0))
    (setv output (get  (get kwargs "outputs") 0))
    (defn split-window [id &optional [vertical False]]
      (+ "tmux split-window " (if vertical "-h " "") "-t " id " bash \n"))
    (defn send-keys [id keys]
      (+ "tmux send-keys -t " id " " keys "\n"))
    (defn select-pane [id direction]
      (+ "tmux select-pane "
         (cond
          [(= direction "up") "-U "]
          [(= direction "down") "-D "]
          [(= direction "left") "-L "]
          [(= direction "right") "-R "])
         "-t "
         id
         "\n"))
    (defn run-command [id cmd]
      (send-keys id (shlex.quote (+ cmd "\n"))))
    (+ (.format *start-tmux-session-template* :id id)
       (reduce + (list-comp
                  (if reason-to-split-window
                    (split-window id)
                    "")
                  [reason-to-split-window [(or log error) pmjq_log htop]]))
       ;; Now in lower pane, all horizontal divs have been made
       (if pmjq_log
         (+ (run-command id (+ "tail -f " pmjq_log))
            (select-pane id "up"))
         "")
       ;; Now in middle-lower pane (or still in bottom one if not pmjq_log)
       (if (or error log)
         (+
          (if (and error log)
            (+ (split-window id :vertical True)
               (select-pane id "left"))
            "")
          (if log
            (run-command id (+ "watch -n1 echo \"" log " ; ls -1 " log " | wc -l ; tail " log "/*\""))
            "")
          (if (and error log)
            (select-pane id "right")
            "")
          (if error
            (run-command id (+ "watch -n1 echo \"" error " ; ls -1 " error " | wc -l ; tail " error "/*\""))
            "")
          (select-pane id "up"))
         "")
       ;; Now in middle-upper pane (or only pane if neither pmjq_log nor log nor error nor htop)
       (+ (split-window id :vertical True)
          (select-pane id "left")
          (run-command id (+ "watch -n1 \"echo " input " ; ls -1 " input " | wc -l ; tail " input "/*\""))
          (select-pane id "right")
          (run-command id (+ "watch -n1 \"echo " output " ; ls -1 " output " | wc -l ; tail " output "/*\""))
          (if htop
            (select-pane id "up")))
       ;; Now in upper plane
       (if htop
         (run-command id "htop")
         "")
       ))
  (eval transition-sexpr))

;; (defn filter [input cmd output]
;;   "Returns the filter as a pmjq command line"
;;   (transition
;;    :inputs [input]
;;    :outputs [output "$0"]
;;    :cmd cmd
;;    :id cmd))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;(def *ssh-user* "edouard")
;(def *ssh-host* "172.20.11.242")
;(def *usual-dirs* ["input" "output" "error" "logs"])
;(def *spool-dir* "/var/spool/dl/")
;(def *mount-template* "TMP_{dir}_DIR=$(mktemp -d) || exit 1
;sshfs {user}@{host}:{remote_dir} $TMP_{dir}_DIR
;")
;(def *transition*
;  '(transition
;    :error "$TMP_ERROR_DIR"
;    :log "$TMP_LOG_DIR"
;    :inputs ["$TMP_INPUT_DIR"]
;    :outputs ["$TMP_OUTPUT_DIR"]
;    :cmd "dl"
;    :pmjq-log "$(date -u +\"%Y-%m-%dT%H:%M:%SZ\")_$(hostname)_pmjq.log")
;  )

;(print "#!/usr/bin/env bash
;set -e
;set -u
;set -x
;set -o pipefail
;")

;(print (.join "\n" (list-comp
;                    (.format *mount-template*
;                             :dir (.upper dir)
;                             :user *ssh-user*
;                             :host *ssh-host*
;                             :remote-dir *spool-dir*)
;                    (dir *usual-dirs*))))

;(print (+ (pmjq-command *transition*) "&"))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(print "#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

export PLAYGROUND=/tmp
export MD5_CMD=md5sum

rm -rf ${PLAYGROUND}/input
rm -rf ${PLAYGROUND}/output
rm -rf ${PLAYGROUND}/error
rm -rf ${PLAYGROUND}/log

mkdir -p ${PLAYGROUND}/input
mkdir -p ${PLAYGROUND}/output
mkdir -p ${PLAYGROUND}/error
mkdir -p ${PLAYGROUND}/log

for file in $(seq 10000)
do
    echo $file > ${PLAYGROUND}/input/$file.txt
done

cd \"$(dirname \"$0\")\"
")

(def *transition* '(transition
                    :quit-empty True
                    :inputs ["${PLAYGROUND}/input"]
                    :cmd "${MD5_CMD}"
                    :outputs ["${PLAYGROUND}/output"]
                    :error "${PLAYGROUND}/error"
                    :log "${PLAYGROUND}/log"
                    :pmjq-log "${PLAYGROUND}/pmjq.log"
                    :id "Test_pmjq")
     )
(print (pmjq-tmux-supervision *transition*))
(print (+ (pmjq-command *transition*) " \n"))

