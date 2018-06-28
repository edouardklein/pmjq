transitions = [{
    'quit_empty': True,
    'stdin': "${PLAYGROUND}/input/'.*'",
    'cmd': "${MD5_CMD}",
    'stdout': "${PLAYGROUND}/output/",
    'log': "${PLAYGROUND}/pmjq.log",
}]
