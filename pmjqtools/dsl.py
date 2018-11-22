'''We have developped a DSL to help users design complex data processing
pipelines.

The atomic unit of a data processing pipeline is the
*transition*: a single command that defines a step in
the pipeline.

Our DSL consists of Python dictionaries (one per transition)
that roughly maps to the command line options of the pmjq executable
(`Basic Usage`_).


Examples
==========

Simplest example
-----------------

.. literalinclude:: smallest_transition.py
    :language: python

.. figure:: smallest_transition.png
   :alt: A visualization of the smallest possible transition

This is the simplest example of a transition: every time a file is dropped
in /tmp/input, its sha256sum will be computed and put in a file of the same
name in /tmp/output. The input file will be deleted.


This code can be saved in, e.g. a ``transition.py`` file and put under version
control.
One can use all the functionalities of Python to dynamically determine
what to put in the dictionaries (e.g. decide, given the current load, whether
to have a transition work from a disk or from a RAM filesystem).

Most complete example
----------------------

.. literalinclude:: complete_transition.py
    :language: python

.. figure:: complete_transition.png
   :alt: A visualization of the most complete transition


This transition uses all the advanced features of PMJQ:
 * Mutliple inputs
    * Files in the two input dirs are matched with one another depending\
      on part of their filenames, as specified in their respective regexes.
 * Multiple outputs
    * Whose filenames depend on the input filenames
 * Error dirs to copy the input files to in case of a failure of the command
 * Documented side-effects of the command
    * PMJQ can not possibly know what the command it is running does.\
      The side effects are just here for documentation purposes.
      As a rule of thumb, one should avoid them.
 * Log dir to store the standard error of the command
    * The log file's name depends on both imput file's names
 * PMJQ will stop when the input dirs are empty (useful for tests or one-off\
   processing)


Bash script generation
========================


One can generate a the bash command that will start a transition with:

.. code-block:: python

    from pmjqtools.dsl import pmjq_command
    transition = {'stdin': '/tmp/input/',
                  'stdout': '/tmp/output/',
                  'cmd': 'sha256sum'}
    print(pmjq_command(transition))


Or, as a one-liner:

.. command-output:: pmjq_cmd --exec smallest_transition.py\
 '[smallest_transition]'


DataFlow Visualization
=======================

The pmjq_viz command can be used to generate a .dot file that can be further
processed as an image.

.. command-output:: pmjq_viz --exec smallest_transition.py\
 '[smallest_transition]'


Using `entr`, one can make change to one or more transition and visualize the
changes in near real-time.

.. code-block:: bash

    $picviewer_with_autoreload smallest_transition.png&
    echo *.py | entr sh -c "pmjq_viz --exec smallest_transition.py\
    '[smallest_transition]' \
    | dot -Tpng > smallest_transition.png"


GNU Shepherd Service
=====================

The pmjq_herd command can be used to generate a .scm file that can then be loaded
by GNU shepherd.

.. command-output:: pmjq_herd --exec smallest_transition.py\
 '[smallest_transition]'


Remote vs. local usage
========================


If one only uses the DSL to generate commands and visualize the pipeline,
then the `pmjqtools`
package need only be installed on the designer's workstation. The
computer that will actually run the pipeline only need the
static `pmjq` binary and a way to remotely run the generated commands
(typically `ssh`).

It can be easier to use the more advanced tools described in
:py:mod:`pmjqtools.tools` on the remote machine, with the drawback of
having to install the `pmjqtools` package on the remote machine.

DSL API Reference
==================
'''

from collections import defaultdict
from string import Formatter
import os
from docopt import docopt
import subprocess
from jinja2 import Template
import shlex


def smart_unquote(string):
    result = subprocess.check_output("echo -n "+string, shell=True)
    return result.decode()


def normalize(transition, root=None):
    "Allow omitting id and the use of shortcuts like stdin, stdout and error"
    assert ("stdin" in transition) != ("inputs" in transition), \
        "Use either stdin or inputs but not both nor neither"  # XOR
    assert ("stdout" in transition) != ("outputs" in transition), \
        "Use either stdout or outputs but not both nor neither"  # XOR
    assert not (("error" in transition) and ("errors" in transition)), \
        "Use either error or errors or neither but not both"  # NAND
    assert 'cmd' in transition, "Every transition needs a command"
    if "stdin" in transition:
        transition["inputs"] = [transition["stdin"]]
        del transition['stdin']
    if "stdout" in transition:
        transition["outputs"] = [transition["stdout"]]
        del transition['stdout']
    if "error" in transition:
        transition["errors"] = [transition["error"]]
        del transition['error']
    if 'id' not in transition:
        transition['id'] = transition['cmd']
    root = os.path.abspath(root) if root is not None else ""
    for key in [k for k in ['inputs', 'outputs', 'errors'] if k in transition]:
        for i in [x for x in range(len(transition[key]))
                  if not os.path.isabs(transition[key][x])]:
            transition[key][i] = os.path.join(root, transition[key][i])
    for key in [k for k in ["log", "stderr"] if k in transition]:
        if not os.path.isabs(transition[key]):
            transition[key] = os.path.join(root, transition[key])
    return transition


def endpoints(transition):
    """Return the list of endpoints: the dirs that need to exist for the transition
    to run"""
    return list(map(lambda d: os.path.dirname(smart_unquote(d)),
                    sum([transition[x] for x in ['inputs', 'outputs', 'errors']
                         if x in transition], []) +
                    ([transition['stderr']] if 'stderr' in transition else [])))


def pmjq_command(transition, redirect="&>"):
    "Return the shell command that will make pmjq run the given transition"
    transition = normalize(transition)
    answer = "pmjq "
    if "quit_empty" in transition and transition["quit_empty"]:
        answer += "--quit-when-empty "
    answer += " ".join(map(lambda pattern: "--input="+pattern,
                           transition["inputs"])) + " "
    if "invariant" in transition:
        answer += "--invariant="+transition["invariant"]+" "
    answer += transition["cmd"]+" "
    answer += " ".join(map(lambda template: "--output="+template,
                           transition["outputs"]))+" "
    if "stderr" in transition:
        answer += "--stderr="+transition["stderr"]+" "
    if "errors" in transition:
        answer += " ".join(map(lambda tmplt: "--error="+tmplt,
                               transition["errors"]))+" "
    if "log" in transition:
        answer += redirect + " " + transition["log"]
    return answer


def pmjq_shepherd(transition):
    "Return the guile code of a shepherd service for the given transition"
    logless_t = transition.copy()  # We need the command without the redirect
    if 'log' in transition:
        del logless_t['log']
    cmd = pmjq_command(logless_t)
    return HERD_BODY.render(id=transition['id'],
                            endpoints=endpoints(transition),
                            cmd_args='"' + '" "'.join(shlex.split(cmd)) + '"',
                            log=transition.get('log', None))


EDGE_TEMPLATE = '''
{{ "{node1}" [shape="{shape1}", color="{color1}", label="{label1}"]}}
->
{{"{node2}" [shape="{shape2}", color="{color2}", label="{label2}"]}}
[arrowhead="{ahead}", style="{astyle}",color="{acolor}",weight={aweight}];
'''


def dot_nodes_and_edge(**kwargs):
    """Return the dot code for two nodes and an edge"""
    return Formatter().vformat(EDGE_TEMPLATE, (),
                               defaultdict(lambda: "", **kwargs))


def trans2dot(transition, include_logs=False):
    """Transform a pmjq transition into a dot string.

    :param transition: The transition to convert
    :param include_logs: Include the 'logs' field
    :return: The dot string, and the list of 'error' fields\
    (for further processing)
    """
    dot = ""
    tr = transition.get("id")
    for i in transition.get("inputs", []):
        name = os.path.dirname(smart_unquote(i))
        dot += dot_nodes_and_edge(node1='dir_'+name, shape1="oval",
                                  color1="blue", label1=name,
                                  node2=tr, shape2="box",
                                  color2="green", label2=tr,
                                  ahead="normal", acolor="blue", aweight="10")
    for o in transition.get("outputs", []):
        name = os.path.dirname(smart_unquote(o))
        dot += dot_nodes_and_edge(node1=tr, shape1="box",
                                  color1="green", label1=tr,
                                  node2='dir_'+name, shape2="oval",
                                  color2="blue", label2=name,
                                  ahead="normal", acolor="blue", aweight="10")
    errors = []
    for e in transition.get("errors", []):
        name = os.path.dirname(smart_unquote(e))
        dot += dot_nodes_and_edge(node1=tr, shape1="box",
                                  color1="green", label1=tr,
                                  node2='dir_'+name, shape2="hexagon",
                                  color2="red", label2=name,
                                  ahead="none", astyle="dotted", acolor="red",
                                  aweight="1")
        errors.append('"dir_{}"'.format(name))
    for s in transition.get("side_effects", []):
        dot += dot_nodes_and_edge(node1=tr, shape1="box",
                                  color1="green", label1=tr,
                                  node2='se_'+s, shape2="diamond",
                                  color2="purple", label2=s,
                                  ahead="none", acolor="purple", aweight="1")
    logs = transition.get("stderr", "")
    if logs and include_logs:
        logs = os.path.dirname(smart_unquote(logs))
        dot += dot_nodes_and_edge(node1=tr, shape1="box",
                                  color1="green", label1=tr,
                                  node2='dir_'+logs, shape2="octagon",
                                  color2="darkorange", label2=logs,
                                  ahead="none", acolor="darkorange",
                                  aweight="2")
    return dot, errors


def transitions2dot(transitions, group=True, logs=False):
    """Convert a pmjq transition list into a dot string.

    :param transitions: The list of transitions
    :param group: Group errors in a cluster
    :return: The dot string
    """
    dot = "digraph transitions {\n"
    dot += "    rankdir = TB;\n"
    dot += "    splines=ortho;\n"
    dot += "    node[penwidth=2.0];\n"
    errors = []
    for t in transitions:
        d, err = trans2dot(normalize(t), include_logs=logs)
        dot += d
        errors += err
    if group and (len(errors) > 0):  # Dirty hack to gather errors
        dot += "subgraph cluster_errors {{\n"\
               "color=none\nedge[style=invis]\n{}\n"\
               "}}\n".format(" -> ".join(errors))
    dot += "}"
    return dot


def run_on_transitions_from_cli(arguments, func, root=None):
    """Call func with the transitions given on the cli.

    The user can specify the transitions using an optional <exec.py>
    argument, which specify some code to run (typically setting a variable),
    and a <eval> argument, whose eval()uation gives the transitions.

    In Python 3, exec() can no longer modify the local variables:
    https://stackoverflow.com/questions/1463306/how-does-exec-work-with-locals

    """
    ldict = locals()
    if arguments['--exec'] is not None:
        exec(open(arguments['--exec']).read(), globals(), ldict)
    transitions = [normalize(t, root)
                   for t in eval(arguments['<eval>'], globals(), ldict)]
    return func(transitions)


def viz():
    """pmjq_viz: Visualize a list of transition as a dot graph

    Usage:
    pmjq_viz [--exec=<exec.py>] [--logs] <eval>

    Options:
      --exec=<exec.py>       Specify a Python file to be exec()d before <eval>
                             is eval()ed.
      --logs                 Draw the logs directories.


    """
    arguments = docopt(viz.__doc__, version='pmjq_viz 1.0.0β')
    run_on_transitions_from_cli(
        arguments,
        lambda trans: print(transitions2dot(trans,
                                            logs=arguments['--logs'])))


def cmd():
    """pmjq_cmd: print the pmjq commands that will run the given transitions.

    Usage:
    pmjq_cmd [--exec=<exec.py>] <eval>

    Options:
      --exec=<exec.py>       Specify a Python file to be exec()d before <eval>
                             is eval()ed
    """
    arguments = docopt(cmd.__doc__, version='pmjq_viz 1.0.0β')

    def print_commands(transitions):
        for t in transitions:
            print(pmjq_command(t))
    run_on_transitions_from_cli(arguments, print_commands)

HERD_HEADER=Template("""(define-module (gnu services toto)
  #:use-module (shepherd service)
  #:use-module (shepherd support)
  #:use-module (oop goops)
  #:export ({% for id in ids %}{{id}}-service {% endfor %}))

""")

HERD_BODY=Template("""(define {{id}}-service
  (make <service>
    #:docstring "PMJQ Transition {{id}}"
    #:provides '({{id}})
    #:requires '()
    #:respawn? #t
    #:start
    (lambda args
      (map (lambda (endpoint)
             (unless (file-exists? endpoint)
               (local-output (string-append "Creating missing endpoint " endpoint))
               (mkdir-p endpoint)))
           '({% for ep in endpoints %}"{{ep}}" {% endfor %}))
      ((make-forkexec-constructor
        (list
         {{cmd_args}})
        {% if log %}
        #:log-file "{{log}}"
        {%- endif -%}
        )))
    #:stop
    (lambda (running . args)
      ((make-kill-destructor 2) ;SIGINT
       (slot-ref {{id}}-service 'running))
	    #f)
    #:actions
    (make-actions
     (help
      "Show the help message"
      (lambda _
        (local-output "This service can start, stop and display the status of the {{id}} pmjq transition."))))))
""")

HERD_FOOTER=Template("""
(register-services {% for id in ids %}{{id}}-service {% endfor %})
""")


def herd():
    """pmjq_shepherd: print a guile description of a shepherd service that will run
    the given transitions

    Usage:
    pmjq_herd [--exec=<exec.py>] <eval>

    Options:
      --exec=<exec.py>       Specify a Python file to be exec()d before <eval>
                             is eval()ed
    """
    arguments = docopt(cmd.__doc__, version='pmjq_herd 1.0.0β')

    def print_services(transitions):
        print(HERD_HEADER.render(ids=[t['id'] for t in transitions]))
        for t in transitions:
            print(pmjq_shepherd(t))
        print(HERD_FOOTER.render(ids=[t['id'] for t in transitions]))
    run_on_transitions_from_cli(arguments, print_services)
