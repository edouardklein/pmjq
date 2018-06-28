transitions = [{
    'quit_empty': True,
    'stdin': "${PLAYGROUND}/input/'.*'",
    'cmd': "'grep -v error'",
    'error': "${PLAYGROUND}/error/",
    'stdout': "${PLAYGROUND}/output/",
    'log': "${PLAYGROUND}/pmjq.log",
}]
