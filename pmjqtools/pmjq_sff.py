"""Some users are so technologically disabled that they can't even mount a
remote
directory. To help them use the dataflows made and run with `pmjq` we
developped
a Web UI. We discourage its use, as for this use case web technologies are
inferior
in almost every way to remote filesystems.
Sadly sometimes empowering the users is not an option and one has to
dumb the tool down.

Usage
------

The ``serve`` target of the Makefile gives a quick overview of what you need
in order to expose the Sigle File Flow (SFF) to the world:

.. command-output:: make -C .. -n serve

.. todo::
    * Suggest ways to adapt the web UI to a particular use case.

Architecture
-------------

The `pmjq_sff` command (:py:func:`sff`) provides the backend for the Web UI.
It is started by `websocketd <http://websocketd.com/>`_, which will communicate
with `pmjq_sff` using its standard input and output.

The front-end is written in ClojureScript. The front-end communicates with
the backend using the websocket opened by websocketd.

.. todo::
    * Make a pretty picture describing this architecture
    * Link to the documentation of the CLJS code

Protocol
---------

The font-end and the backend communicate using a text protocol read and written
from the front-end using a websocket, and from the backend using stdout and
stdin,
thanks to the translation made by websocketd.

Quick description:

* The protocol is text based.
* All messages are one line long.
* They are typed by a prefix followed by ':'
* The front end never initiates an exchange, it only answers the backend.
* Not all messages from the backend require an answer.



.. list-table:: PMJQ UI protocol
   :widths: 5 5 5 30 20
   :header-rows: 1

   * - Direction
     - Type
     - payload
     - Meaning
     - Example
   * - ``<-``
     - input
     - <dir_name>
     - The backend informs the frontend that an input directory named\
       <dir_name> exists.
     - <- ``input:/tmp/audio``
   * - ``->``
     - data
     - <dir_name>:<file_data_in_base_64>
     - The frontend gives the data to be written in <dir_name>
     - -> ``data:/tmp/audio:c7bc...5a93``
   * - ``->``
     - name
     - <dir_name>:<file_name>
     - The frontend informs the backend of the name the file had on the user's\
       machine.
     - -> ``name:/tmp/audio/:toto.wav``
   * - ``<-``
     - log
     - <one log line>
     - All pmjq log lines that are relevant to the user's files are sent\
       this way
     - <- ``log:2018/04/30 17:24:14 pmjq.go:747: 000000 free...``
   * - ``<-``
     - waiting
     - <dir_name>:<n>
     - The back-end tells the front-end that `n` files are waiting to be
       processed in `dir_name`.
     - <- ``waiting:/tmp/input:42``
   * - ``<-``
     - stderr
     - <fname>:<error_message>
     - The backend tells the frontend that one command failed a provides \
       a b64-encoding of the standard error of the command that failed, along\
       with the file name where this stderr was dumped.
     - <- ``stderr:/tmp/err/toto.txt.log:abd54...g6a67``
   * - ``<-``
     - output
     - <URL of a file>
     - The backend tells the frontend that an output file is ready to download
     - <- ``output:http://example.com/toto.txt``
   * - ``<-``
     - done
     - N/A
     - The backend tells the frontend that the processing is over and no more\
       messages will be coming.
     - <- ``done``


.. todo::

    FIXME: This table will quickly get out of sync with the code, which is
    actually split in three places:

    * The Golang code that emits the logs
    * The Python code in the Backend that parses those logs and sends the \
      messages
    * The CLJS code in the frontend that receives the messages.

    We should generate the table from a piece of doc much closer to the code.


"""

import os
from os.path import join as pjoin
from os.path import abspath
from docopt import docopt
from chan import quickthread as go
from chan import Chan
import inotify.adapters
import inotify.constants
import base64
import random
import tempfile
from datetime import datetime as dt
import sys
from .dsl import run_on_transitions_from_cli, smart_unquote
from collections import defaultdict


def send(*args):
    "Print and flush. No flush leads to the front- and back-end deadlocking."
    sys.stdout.write("".join(args + ("\n",)))
    sys.stdout.flush()
    return


def get_default_filter(pattern):
    "Look for pattern in the line"
    return lambda log: pattern in log


def default_handler(log):
    "Send the log line to the frent-end"
    send("log:", log)


def tail_filter_and_process(fname, filterfunc,
                            handlerfunc=default_handler, **kwargs):
    """Watch a log file, filter the lines, process the matching lines.

    If the filter function accepts the line, it will be processed by
    the handler function.

    :param fname: The file to watch
    :param filterfunc: A boolean function of the log line to filter the lines\
        containing the pattern
    :param handlerfunc: A function to be called on each selected line
    :param kwargs: Additionnal keywords arguments that will be passed to\
        handlerfunc

    """
    sys.stderr.write("Reading log: "+str(fname)+"\n")
    with open(fname, "r") as f:
        f.seek(0, 2)  # Go to EOF
        while True:
            log = f.readline().strip()
            if not log or not filterfunc(log):
                continue
            # send("calling", str(handlerfunc))
            handlerfunc(log, **kwargs)


def get_error_filter(pattern):
    "Match the log message PMJQ emits when a data processing command fails"
    # TODO Possibly improve matching
    return lambda log: pattern in log and "ERROR Rejected file from " in log


def error_handler(log, root=""):
    """Send the error message of the failing data processing command to the
    front-end"""
    logp = log.split(" ")
    sourcefile, logfile = logp[-4], logp[-1].strip("()")
    with open(pjoin(root, logfile), "r") as ef:
        message = ef.read()
    directory = os.path.dirname(sourcefile)
    message = base64.encodebytes(message.encode("utf-8"))
    message = message.decode().replace("\n", "").strip()
    send("stderr:", os.path.normpath(pjoin(root, directory)), ":", message)


# INFO Candidates for %v:%v
def waiting_filter(log):
    """Match the message PMJQ emits when it knows how many files are
    waiting for processing"""
    return "INFO Candidates for " in log


def waiting_handler(log, root=""):
    directory, nb = log.split(" ")[-1].split(":")[-2:]
    send("waiting:", os.path.normpath(pjoin(root, directory)), ":", nb)


def move_and_print(directory, pattern, outdir, url, ch, prefix=""):
    """Print the name of files created in directory
    that contain pattern.
    """
    # TODO Multiple watches instead of for loop?
    i = inotify.adapters.Inotify()
    i.add_watch(directory)
    for event in i.event_gen():
        if event is None:
            continue
        (header, type_names, watch_path, filename) = event
        # Look for PMJQ lock release
        if (pattern in filename
                and filename.endswith(".lock")
                and 'IN_DELETE' in type_names):
            filpath = pjoin(directory, filename[:-5])
            if os.path.exists(filpath):
                assert os.path.exists(outdir), "No DL folder!"
                newname = "{}-{}".format(prefix, filename[:-5])
                outfile = os.path.abspath(pjoin(outdir, newname))
                os.rename(filpath, outfile)
                send("output:" + pjoin(url, newname))
                ch.put("done")
                return


def handle_inputs(inputs, job_id):
    """Receive file inputs (name and data) and send them in the
    appropriate folders

    :param inputs: The list of input folders
    :param job_id: A unique prefix to avoid name collisions
    """
    received_so_far = defaultdict(lambda: {})
    for i in range(len(inputs)*2):  # Name + Data
        msg = input()
        msg_type, dirname, content = msg.split(":", 2)
        assert msg_type in ("name", "data"), "Wrong type"
        received_so_far[dirname][msg_type] = content
        if "name" in received_so_far[dirname] \
           and "data" in received_so_far[dirname]:
            fname = job_id + os.path.splitext(
                received_so_far[dirname]['name'])[-1]
            with tempfile.NamedTemporaryFile(delete=False) as f:
                f.write(base64.decodebytes(
                    received_so_far[dirname]['data'].encode('utf8')))
                tname = f.name
            os.rename(tname, pjoin(dirname, fname))


def find_leaves(trans):
    """Return the 'leaves' of the DAG formed by the given transitions.

    The transitions form a Directed Acyclic Graph. We call leaves
    those nodes whose edges are all of the same direction.

    Nodes with only outgoing edges are input nodes.
    Nodes with only incoming edges are output nodes.

    Nodes with both incoming and outgoing edges are
    intermediary nodes who are supposed to keep files just
    long enough for them to be processed by the next transition.

    This may not be true if the dataflow designer played with the
    filenames regexes and patterns in a way that will make files
    permanently stay in an intermediary node.
    We discourage such designs as stupidly unclear.

    :param trans: The transitions
    :return: Three lists: input directories, output directories, log files
    """
    inputs = set(sum([t['inputs'] for t in trans], []))
    outputs = set(sum([t['outputs'] for t in trans], []))
    logs = [t['log'] for t in trans if 'log' in t]

    inputs = {os.path.normpath(os.path.dirname(smart_unquote(i)))
              for i in inputs}
    outputs = {os.path.normpath(os.path.dirname(smart_unquote(i)))
               for i in outputs}
    logs = {os.path.normpath(smart_unquote(i)) for i in logs}
    return inputs - outputs, outputs - inputs, logs


def main(args):
    """
    Backend for the Web interface to PMJQ.

    Usage:
        pmjq_sff [--exec=<exec.py>] --dl-folder=<folder>
        [--root=<root>] --dl-url=<url> <eval>

    Options:
      --exec=<exec.py>      Specify a Python file to be exec()d before <eval>
                            is eval()ed.
      --root=<root>         Working directory to evaluate other relative paths
                            from.
      --dl-folder=<folder>  Directory where the output files will be made
                            available to the users.
      --dl-url=<url>        Publicly accessible URL of <folder>
    """
    root = os.path.normpath(abspath(args['--root'])) \
        if args['--root'] is not None else None

    if os.path.isabs(args['--dl-folder']):
        dlfolder = args['--dl-folder']
    elif root is not None:
        dlfolder = pjoin(root, args['--dl-folder'])
    else:
        dlfolder = os.path.abspath(args('--dl-folder'))

    dlurl = args['--dl-url']
    nonce = str(random.randint(0, 10000)).zfill(4)
    # The job id is the name of the file, the extension will come from the user
    job_id = dt.now().isoformat() + "_" + nonce

    inputs, outputs, logs = run_on_transitions_from_cli(
        args, find_leaves, root=root)

    for indir in inputs:  # Write input files
        assert ':' not in indir, \
            "FIXME: (later) ':' in dir names not supported yet"
        send("input:" + indir)
    sys.stderr.write("Watching log files: ")
    sys.stderr.write(str(logs)+"\n")
    for logfile in logs:
        go(tail_filter_and_process, fname=logfile,
           filterfunc=get_default_filter(job_id),
           handlerfunc=default_handler)
        go(tail_filter_and_process, fname=logfile,
           filterfunc=waiting_filter,
           handlerfunc=waiting_handler,
           root=root or "")
        go(tail_filter_and_process, fname=logfile,
           filterfunc=get_error_filter(job_id),
           handlerfunc=error_handler,
           root=root or "")
    i = 0
    ch = Chan()
    sys.stderr.write("Watching output dirs: ")
    sys.stderr.write(str(outputs)+"\n")
    for outdir in outputs:
        prefix = os.path.basename(os.path.normpath(outdir)) + str(i)
        i += 1
        go(move_and_print, outdir, job_id,
           dlfolder, dlurl, ch, prefix)
    go(handle_inputs, inputs, job_id)
    # Wait for output to be processed
    for _ in range(len(outputs)):
        ch.get()
    return


def launch():
    arguments = docopt(main.__doc__)
    main(arguments)
