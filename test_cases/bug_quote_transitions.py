from shlex import quote as q

transitions = [
    {
        "id": "transition",
        "error": q('errors/{{.Input 0}}'),
        "stderr": q('logs/{{.Input 0}}'),
        "inputs": [q('input/(?P<id>.*).*')],
        "outputs": [q('output/{{.Input 0}}')],
        "cmd": "echo",
    }
]
