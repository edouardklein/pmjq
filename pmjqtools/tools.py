'''
Tools
======

``pmjq_interactive``
---------------------

The ``pmjq_interactive`` tool generates three shell scripts.
 - The first one (``setup.sh``) creates the directories, users and groups and\
sets the permissions.
 - The second one (``launch.sh``) launches the daemons.
 - The last one (``cleanup.sh``) removes the directories, users and groups.

The user is asked, in order:
 - which command ``pmjq`` is supposed to launch,
 - which are the input directories,
 - which are the output directories.

This tuple ``(input, cmd, output)`` is called an `invocation`, because it will,
in ``launch.sh``, map to one invocation of ``pmjq``.

A transcript of a user setting up the first two invocations of our example
pipeline using this tool would begin like this::


    Command:decode
    Input dir(s):input/
    Output dir(s):decoded/
    Command:stabilize
    Input dir(s):decoded/
    Output dir(s):stabilized/


The generated scripts may have to be tweaked afterwards to add
mounting/unmounting of remote, RAM or virtual file systems, or to add options
(e.g. ``--max-load``) to the invocations of ``pmjq``. They lend themselves
quite well to merging, diffing and version control.

The ``pmjq_interactive`` tool can also be used non interactively, by storing
in a file the answers one would give to the tool's questions. Comments can be
added by starting a line with a #, such as:

.. literalinclude:: ../paper/whole_pipeline.txt

Such a file may be used on the command line with::

    grep -v -E '^#' < whole_pipeline.txt | ../pmjq_interactive

'''
from collections import namedtuple

Invocation = namedtuple('Invocation', 'inputs,command,outputs,pattern')

INVOCATIONS = set()
'''The INVOCATIONS set grows as the user inputs data, it then processed.'''


def invocation_name(invocation):
    '''Return a short unique name for the given invocation'''
    global invocations
    short_name = invocation.command.split(' ')[0]
    if sum([i.command.startswith(short_name) for i in invocations]) < 2:
        return short_name
    return short_name+'_'+str(abs(hash(' '.join(invocation.inputs) +
                                       invocation.command +
                                       ' '.join(invocation.outputs))))[:3]

