def normalize(transition):
    "Allow the use of shortcuts like stdin, stdout and error"
    assert ("stdin" in transition) != ("inputs" in transition), \
        "Use either stdin or inputs but not both nor neither"
    assert ("stdout" in transition) != ("outputs" in transition), \
        "Use either stdout or outputs but not both nor neither"
    assert not (("error" in transition) and ("errors" in transition)), \
        "Use either error or errors but not both (but you can use neither)"
    if "stdin" in transition:
        transition["inputs"] = [transition["stdin"]]
    if "stdout" in transition:
        transition["outputs"] = [transition["stdout"]]
    if "error" in transition:
        transition["errors"] = [transition["error"]]
    return transition


def pmjq_command(transition):
    """Return the shell command that will make pmjq run the given transition."""
    transition = normalize(transition)
    answer = "pmjq "
    if "quit_empty" in transition and transition["quit_empty"]:
        answer += "--quit-when-empty "
    answer += " ".join(map(lambda pattern: "--input="+pattern,
                           transition["inputs"]))
    answer += " "
    if "invariant" in transition:
        answer += "--invariant="+transition["invariant"]+" "
    answer += transition["cmd"]+" "
    answer += " ".join(map(lambda template: "--output="+template,
                           transition["outputs"]))
    if "stderr" in transition:
        answer += "--stderr="+transition["stderr"]+" "
    if "errors" in transition:
        answer += " ".join(map(lambda template: "--error="+template,
                               transition["errors"]))+" "
    if "log" in transition:
        answer += "&> " + transition["log"]
    return answer

print(pmjq_command({"id": "DL_pool", "error":"/var/spool/dl/error/",
                    "stderr": "/var/spool/dl/log", "stdin":"/var/spool/dl/input/",
                    "stdout":"/var/spool/dl/output/", "cmd":"\"sh -c 'read url && curl \\\"\$url\\\"'\"",
                    "quit_empty": True, "log":"/var/spool/dl/pmjq.log"}))
