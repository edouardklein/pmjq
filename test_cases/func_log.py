transitions = [{
    'quit_empty': True,
    'stdin': "${PLAYGROUND}/input/'.*'",
    'cmd': "\"${EXAMPLE_COMMAND}\"",
    'stderr': "${PLAYGROUND}/log/",
    'error': "${PLAYGROUND}/error/",
    'stdout': "${PLAYGROUND}/output/",
    'log': "${PLAYGROUND}/pmjq.log",
}]
