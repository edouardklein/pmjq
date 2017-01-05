(import shlex)
(import [pmjqtools.dsl [*]])
;; The Hystring everywhere are a workaround for  https://github.com/hylang/hy/issues/1174

(print (pmjq-command `(transition
                       :quit-empty True
                       :stdin "${PLAYGROUND}/input/'.*'"
                       :cmd "'grep -v error'"
                       :error "${PLAYGROUND}/error/"
                       :stdout "${PLAYGROUND}/output/"
                       :log "${PLAYGROUND}/pmjq.log"
                       )))

