'''
Tools
======

``pmjq_interactive`` tutorial
-----------------------------

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


``pmjq_interactive`` Code organization
---------------------------------------

If you want to dig in the code, or have a better inderstanding of how this
tool structures the directory, groups, users and rights, read on.


'''
from collections import namedtuple

Invocation = namedtuple('Invocation', 'inputs,command,outputs,pattern')
Invocation.__doc__ = 'This data structure represents one invocation of pmjq'

INVOCATIONS = set()
'''The INVOCATIONS set grows as the user inputs data, it then is processed.'''


def invocation_name(invocation):
    '''Return a short unique name for the given invocation'''
    global invocations
    short_name = invocation.command.split(' ')[0]
    if sum([i.command.startswith(short_name) for i in invocations]) < 2:
        return short_name
    return short_name+'_'+str(abs(hash(' '.join(invocation.inputs) +
                                       invocation.command +
                                       ' '.join(invocation.outputs))))[:3]


def read_user_input():
    '''Prompt the user, create invocations and return them'''
    answer = set()
    while True:
        command = input('Command:')
        if command == '':
            break
        inputs = tuple(sorted(input('Input dir(s):').split(' ')))
        pattern = '(.*)'
        if len(inputs) > 1:
            pattern = input('Pattern (default "(.*)"):') or pattern
        outputs = tuple(sorted(input('Output dir(s):').split(' ')))
        answer.add(Invocation(inputs, command, outputs, pattern))
    return answer


def dir2groups(invocations):
    '''Return the groups and directories that ought to be created

    First, there is one group per input directory.

    Then, there is one group per global output directory.

    A global output directory is a directory no pmjq invocation reads from,
    and in which, consequently, data will accumulate (presumably to be used
    by a human).

    The returned values is a dictionary that maps directory names to the group
    that own them.
    '''
    directories = set(d for i in invocations for d in i.inputs) |\
        set(d for i in invocations for d in i.outputs)
    output_dirs = directories - \
        set(d for i in invocations for d in i.inputs)
    dir2groups = {d: 'pg_'+d for d in output_dirs}
    for i in invocations:
        if len(i.inputs) > 1:
            for j, d in enumerate(i.inputs):
                dir2groups[d] = 'pg_'+invocation_name(i)+'_'+str(j)
            else:
                dir2groups[i.inputs[0]] = 'pg_'+invocation_name(i)
    return dir2groups


def user2groups(invocations):
    '''Return the users that ought to be created

    There is one user per pmjq invocation.

    This user belongs to the groups that own their input and output dirs.

    The returned value is a dictionary that maps users to the groups they
    belong to.
    '''
    d2g = dir2groups(invocations)
    def groups(i):
        return [d2g[d] for d in ]
    return {'pu_'+invocation_name(i):[]}
