(import shlex)
(import [pmjqtools.dsl [*]])
;; The Hystring everywhere are a workaround for  https://github.com/hylang/hy/issues/1174

(print (pmjq-command `(transition
                       :quit-empty True
                       :stdin "${PLAYGROUND}/input/'.*'"
                       :cmd "${MD5_CMD}"
                       :stdout "${PLAYGROUND}/output/"
                       :log "${PLAYGROUND}/pmjq.log"
                       )))

