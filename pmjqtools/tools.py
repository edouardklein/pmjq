'''We have developped some tools to make running `pmjq` dataflows
easier:

* A tool to create the directories a list of transitions need: \
:py:func:`mkendpoints`
* A tool to start (:py:func:`start`) all the transitions, using `tmux`.

.. todo::
    Create pmjq_restart, pmjq_stop, and pmjq_status using the corresponding
    daemux functions.
'''
from .dsl import normalize, pmjq_command, run_on_transitions_from_cli
from .dsl import smart_unquote, endpoints
import os
import daemux
import shlex
from docopt import docopt


def create_endpoints(transitions):
    """Create the directories needed by the given transitions"""
    for t in transitions:
        t = normalize(t)
        for endpoint in endpoints(t):
            os.makedirs(endpoint,
                        exist_ok=True)


def mkendpoints():
    """pmjq_mkendpoints: Create all the directories needed by the given
    transitions.

    Usage:
    pmjq_mkendpoints [--exec=<exec.py>] <eval>

    Options:
      --exec=<exec.py>       Specify a Python file to be exec()d before <eval>
                             is eval()ed
    """
    arguments = docopt(mkendpoints.__doc__, version='pmjq_viz 1.0.0β')
    run_on_transitions_from_cli(arguments, create_endpoints)


COMMAND_TEMPLATES = {
    "log": "lnav {log}",
    "directories": "for dir in {dirs}; do echo $dir; ls -1 $dir | wc -l; done",
    "stderr": "tail {stderr}/$(ls -1t {stderr} | head -1); ls {stderr}",
    "htop": "htop",
}


def daemux_start(transitions, session="pmjq", shell='sh'):
    """Instantiate the transitions, each in its own tmux window"""
    for t in transitions:
        t = normalize(t)
        commands = []
        # Template "directories" deals with watch-able templates
        # that use a list as input
        for dirs_key in [x for x in ["inputs", "outputs", "errors"]
                         if x in t]:
            commands.append("watch -n1 "+shlex.quote(
                COMMAND_TEMPLATES['directories'].format(
                    dirs=' '.join(
                        map(lambda d:
                            os.path.dirname(smart_unquote(d)),
                            t[dirs_key])))))
        # Template "stderr" deals with the log files
        if "stderr" in t:
            commands.append("watch -n1 "+shlex.quote(
                COMMAND_TEMPLATES['stderr'].format(
                    stderr=os.path.dirname(
                        smart_unquote(t['stderr'])))))
        # The command
        if shell == "sh":
            commands.append(pmjq_command(t))
        elif shell == 'fish':
            commands.append(pmjq_command(t, redirect='^'))
        # The other templates can be used as is
        for k in [k for k in COMMAND_TEMPLATES
                  if k not in ['directories', 'stderr', 'cmd']]:
            if k in t:
                commands.append(COMMAND_TEMPLATES[k].format(**t))
        for i, cmd in enumerate(commands):
            daemux.start(cmd, session=session, window=t['id'], pane=i,
                         layout='tiled')


def start():
    """pmjq_start: Instantiate the transitions, each in its own tmux window.

    Usage:
    pmjq_start [--exec=<exec.py>] [--session=<session>] [--shell=<shell>]
    <eval>

    Options:
      --exec=<exec.py>       Specify a Python file to be exec()d before <eval>
                             is eval()ed
      --session=<session>    Specify the tmux session the tmuw windows will be
                             created in. Defaults to "pmjq".
      --shell=<shell>        Specify the shell the command will be run with.
                             Currently only (ba)sh (the default) and fish are
                             officially supported.
    """
    arguments = docopt(start.__doc__, version='pmjq_viz 1.0.0β')
    f_args = {}
    if arguments['--session'] is not None:
        f_args['session'] = arguments['--session']
    if arguments['--shell'] is not None:
        f_args['shell'] = arguments['--shell']
    run_on_transitions_from_cli(
        arguments,
        lambda trans: daemux_start(trans, **f_args))
