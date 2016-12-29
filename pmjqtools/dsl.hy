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
       (shlex.quote (get kwargs "cmd")) " "
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
OLDIFS=${{IFS}}
IFS=$'\n'
for couple in $(printenv); do
  tmux setenv -t html2json $(cut -d'=' -f1 <<<$couple) $(cut -d'=' -f2- <<<$couple)
done
IFS=${{OLDIFS}}
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

