(import shlex)
(import [pmjqtools.dsl [*]])

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(def *ssh-user* "edouard")
(def *ssh-host* "172.20.11.242")
(def *usual-dirs* ["input" "output" "error" "log"])
(def *spool-dir* "/var/spool/dl/")
(def *mount-template* "sshfs {user}@{host}:{remote_dir} {local_dir}")
(def *transition*
  '(transition
    :id "DL_pool"
    :error "/var/spool/dl/error"
    :log "/var/spool/dl/log"
    :inputs ["/var/spool/dl/input"]
    :outputs ["/var/spool/dl/output"]
    :cmd "sh -c 'read url && curl \"$url\"'"
    :quit-empty True
    :pmjq-log "/var/spool/dl/$(hostname)_pmjq.log"
    :lock ["/var/spool/dl/$(hostname.lock)"]))

(print "#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail
")

;; If not already mounted
(print (.format "if ! mount | grep {spool_dir}; then" :spool-dir *spool-dir*))

(print (.format *mount-template*
                :user *ssh-user*
                :host *ssh-host*
                :remote-dir *spool-dir*
                :local-dir *spool-dir*))

(print "fi")

(print (pmjq-tmux-supervision *transition*))
(print (+ (pmjq-command *transition*) " \n"))

