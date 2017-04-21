import shlex

def normalize(transition):
    "Allow the use of shortcuts like stdin, stdout and error"
    assert ("stdin" in transition) != ("inputs" in transition), \
        "Use either stdin or inputs but not both nor neither"  # XOR
    assert ("stdout" in transition) != ("outputs" in transition), \
        "Use either stdout or outputs but not both nor neither"  # XOR
    assert not (("error" in transition) and ("errors" in transition)), \
        "Use either error or errors or neither but not both"  # NAND
    if "stdin" in transition:
        transition["inputs"] = transition["stdin"]
    if "stdout" in transition:
        transition["outputs"] = transition["stdout"]
    if "error" in transition:
        transition["errors"] = transition["error"]
    return transition


def pmjq_command(transition, redirect="&>"):
    """Return the shell command that will make pmjq run the given transition."""
    transition = normalize(transition)
    answer = "pmjq "
    if "quit_empty" in transition and transition["quit_empty"]:
        answer += "--quit-when-empty "
    answer += " ".join(map(lambda pattern: "--input="+shlex.quote(pattern),
                           transition["inputs"])) + " "
    if "invariant" in transition:
        answer += "--invariant="+transition["invariant"]+" "
    answer += shlex.quote(transition["cmd"])+" "
    answer += " ".join(map(lambda template: "--output="+shlex.quote(template),
                           transition["outputs"]))+" "
    if "stderr" in transition:
        answer += "--stderr="+transition["stderr"]+" "
    if "errors" in transition:
        answer += " ".join(map(lambda tmplt: "--error="+shlex.quote(tmplt),
                               transition["errors"]))+" "
    if "log" in transition:
        answer += redirect + " " + transition["log"]
    return answer
