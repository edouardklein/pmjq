(import shlex)

(defn normalize [kwargs]
  "Allow the use of shortcuts like stdin, stdout and error"
  (assert (!= (in "stdin" kwargs) (in "inputs" kwargs))  ; XOR
          "Use either stdin or inputs but not both nor neither")
  (assert (!= (in "stdout" kwargs) (in "outputs" kwargs))  ; XOR
          "Use either stdout or outputs but not both nor neither")
  (assert (not (and (in "error" kwargs) (in "errors" kwargs)))  ; NAND
          "Use either error or errors but not both (but you can use neither)")
  (if (in "stdin" kwargs)
    (setv (. kwargs ["inputs"]) [(. kwargs ["stdin"])]))
  (if (in "stdout" kwargs)
    (setv (. kwargs ["outputs"]) [(. kwargs ["stdout"])]))
  (if (in "error" kwargs)
    (setv (. kwargs ["errors"]) [(. kwargs ["error"])]))
  kwargs
  )

(defn pmjq-command [transition-sexpr]
  "Return the command to launch in a shell to activate the given transition"
  (defn transition [&kwargs kwargs]
    (setv kwargs (normalize kwargs))
    (+ "pmjq "
       (if (and (in "quit_empty" kwargs) (get kwargs "quit_empty"))
         "--quit-when-empty "
         "")
       (.join " "
              (map (fn [inpattern] (+ "--input=" inpattern)) (. kwargs ["inputs"])))
       " "
       (if (in "invariant" kwargs)
         (+ "--invariant=" (. kwargs ["invariant"]) " ")
         "")
       (get kwargs "cmd") " "
       (.join " "
              (map (fn [outtemplate] (+ "--output=" outtemplate)) (. kwargs ["outputs"]))) " "
       (if (in "stderr" kwargs)
         (+ "--stderr=" (get kwargs "stderr") " ")
         "")
       (if (in "errors" kwargs)
         (+ (.join " "
                   (map (fn [errtemplate] (+ "--error=" errtemplate)) (. kwargs ["errors"])))
            " ")
         "")
       (if (in "log" kwargs)
         (+ "&> " (get kwargs "log"))
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
  tmux setenv -t {id} $(cut -d'=' -f1 <<<$couple) $(cut -d'=' -f2- <<<$couple)
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
  | (log)                   |
  +-------------+-----------+
  | inputs      | outputs   |
  +-------------+-----------+
  | (stderr)    | (errors)  |
  +-------------+-----------+
  | (htop)                  |
  +-------------------------+
Panes between () are optional and will not be created if there is no need for them."
  (defn transition [&kwargs kwargs]
    (setv kwargs (normalize kwargs))
    (setv id (get kwargs "id"))
    (setv stderr (if (in "stderr" kwargs) (get kwargs "stderr") False))
    (setv error (if (in "error" kwargs) (get kwargs "error") False)) ;;FIXME wont work with multiple inputs
    (setv log (if (in "log" kwargs) (get kwargs "log") False))
    (setv input (get  (get kwargs "inputs") 0));;FIXME wont work with multiple inputs
    (setv output (get  (get kwargs "outputs") 0));;FIXME wont work with multiple inputs
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
                  [reason-to-split-window [(or stderr error) log htop]]))
       ;; Now in lower pane, all horizontal divs have been made
       (if htop
         (+ (run-command id "htop")
            (select-pane id "up"))
         "")
       ;; Now in middle-lower pane (or still in bottom one if not pmjq_log)
       (if (or error stderr)
         (+
          (if (and error stderr)
            (+ (split-window id :vertical True)
               (select-pane id "left"))
            "")
          (if stderr
            (run-command id (+ "watch -n1 echo \"" stderr " ; ls -1 " stderr " | wc -l ; tail " stderr "/*\""))
            "")
          (if (and error stderr)
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
       (if log
         (run-command id (+ "lnav " log))
         "")
       ))
  (eval transition-sexpr))

