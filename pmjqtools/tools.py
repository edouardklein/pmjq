'''
The ``pmjq`` UNIX security model
================================

``pmjq`` relies on standard UNIX tooling and permissions to make sure a rogue,
compromised of buggy program can do only limited damage.

A folder should only be read by one invocation at a time (but can be read by
multiple instances of the same invocation).

It is technically possible to have a folder be read by multiple invocation, but
it is not advisable as the behavior is undefined : which invocation will
process a file in the folder is anyone's guess. It also breaks the ``pmjq`` UNIX
security model.

The ``pmjq`` UNIX security model is as follow:

- Each directory is owned by a specific group
- Each invocation (and thus the processes it launches) is run by a specific\
user.
- This user owns all the input directories.
- This user belongs to the groups of the input and output directories of its\
invocation.

That way, if a process launched by an invocation of ``pmjq`` goes awry, it
will at most do damage only in its input and output folders, and nowhere else.

This security model is enforced by using the scripts generated by
``pmjq_interactive``.

Tools
======

``pmjq_interactive``
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

    grep -v -E '^#' < whole_pipeline.txt | pmjq_interactive

``pmjq_viz`` 
--------------------

The ``pmjq_viz`` tool lets you visualize the pipeline.

It analyzes the ``setup.sh`` script generated by the ``pmjq_interactive`` tool
and outputs the resulting graph in dot language. To see it in e.g. pdf, one
can run::

    pmjq_viz < setup.sh | dot -T pdf > whole_pipeline.pdf

The fact that the vizualisation tool works from the ``setup.sh`` script instead
of using the user input given to ``pmjq_interactive`` is an important feature
as one can be sure that what we see is what is going to be executed.

'''
import sys
import re
import hashlib
from collections import namedtuple
from .templates import SETUP_TEMPLATE, MKDIR_TEMPLATE, PMJQ_FILTER_TEMPLATE,\
    PMJQ_BRANCH_MERGE_TEMPLATE, CLEANUP_TEMPLATE, DOT_TEMPLATE

Invocation = namedtuple('Invocation', 'inputs,command,outputs,pattern')
Invocation.__doc__ = 'This data structure represents one invocation of pmjq'


def invocation_name(invocations, inv):
    '''Return a short unique name for the given invocation'''
    short_name = inv.command.split(' ')[0]
    if sum([i.command.startswith(short_name) for i in invocations]) < 2:
        return short_name
    return short_name+'_' + \
        hashlib.md5((' '.join(sorted(inv.inputs)) +
                    inv.command +
                     ' '.join(sorted(inv.outputs)))
                    .encode('utf-8')).hexdigest()[:3]


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


def dir2group(invocations):
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
                dir2groups[d] = 'pg_'+invocation_name(invocations, i)\
                                + '_'+str(j)
        else:
            dir2groups[i.inputs[0]] = 'pg_'+invocation_name(invocations, i)
    return dir2groups


def user2groups(invocations):
    '''Return the users that ought to be created

    There is one user per pmjq invocation.

    This user belongs to the groups that own their input and output dirs.

    The returned value is a dictionary that maps users to the groups they
    belong to.
    '''
    d2g = dir2group(invocations)

    def groups(i):
        return [d2g[d] for d in i.inputs + i.outputs]
    return {'pu_'+invocation_name(invocations, i): groups(i)
            for i in invocations}


def groupadd(invocations):
    '''Return the groupadd part of the setup script'''
    return '\n'.join(sorted(['groupadd '+grp
                             for grp in dir2group(invocations).values()]))


def usermod(invocations):
    '''Return the usermod part of the setup script'''
    return 'usermod -a -G '+','.join(sorted(dir2group(invocations).values()))\
        + ' `whoami`'


def useradd(invocations):
    '''Return the useradd part of the setup script'''
    u2g = user2groups(invocations)

    def useradd_line(user):
        return 'useradd -M -N -g {group} -G {other_groups} {user}'\
            .format(user=user, group=u2g[user][0],
                    other_groups=','.join(u2g[user][1:]))
    return '\n'.join(useradd_line(u)
                     for u in sorted(user2groups(invocations).keys()))+'\n'


def mkdir(invocations):
    '''Return the mkdir part of the setup script'''
    d2g = dir2group(invocations)

    def user(d):
        try:
            invocation = [i for i in invocations if d in i.inputs][0]
        except IndexError:  # d is an output dir
            return '`whoami`'
        return 'pu_' + invocation_name(invocations, invocation)
    return '\n'.join(MKDIR_TEMPLATE.format(directory=d,
                                           group=d2g[d],
                                           user=user(d))
                     for d in sorted(d2g.keys()))


def setup(invocations):
    '''Return the text of the setup script'''
    return SETUP_TEMPLATE.format(groupadd=groupadd(invocations),
                                 usermod=usermod(invocations),
                                 useradd=useradd(invocations),
                                 mkdir=mkdir(invocations))


def launch(invocations):
    '''Return the text of the launch script'''
    def user(i):
        return 'pu_'+invocation_name(invocations, i)

    def command(i):
        if ' ' not in i.command:
            return i.command
        answer = i.command.replace('$', '\$').replace('"', '\"')
        answer = '"'+answer.replace('`', '\`')+'"'
        return answer

    def pmjq_line(i):
        if len(i.inputs) == 1 and len(i.outputs) == 1:  # Filter call
            return PMJQ_FILTER_TEMPLATE.format(user=user(i),
                                               input=i.inputs[0],
                                               filter=command(i),
                                               output=i.outputs[0])
        else:  # Branching or merging call
            return PMJQ_BRANCH_MERGE_TEMPLATE.format(user=user(i),
                                                     pattern=i.pattern,
                                                     inputs=' '.join(i.inputs),
                                                     cmd=command(i))
    return '#!/usr/bin/env sh\n' + \
        '\n'.join(pmjq_line(i) for i in sorted(invocations,
                                               key=lambda i: i.command))+'\n'


def cleanup(invocations):
    '''Return the text of the cleanup script'''
    groupdel = '\n'.join('groupdel '+group
                         for group in sorted(dir2group(invocations).values()))
    userdel = '\n'.join('userdel '+user
                        for user in sorted(user2groups(invocations).keys()))
    rm = '\n'.join('rm -r '+d
                   for d in sorted(dir2group(invocations).keys()))
    return CLEANUP_TEMPLATE.format(groupdel=groupdel,
                                   userdel=userdel,
                                   rm=rm)


def pmjq_interactive():
    '''Entry point of the pmjq_interactive executable'''
    invocations = read_user_input()
    with open('setup.sh', 'w') as f:
        f.write(setup(invocations))
    with open('launch.sh', 'w') as f:
        f.write(launch(invocations))
    with open('cleanup.sh', 'w') as f:
        f.write(cleanup(invocations))


def extract_places(lines):
    '''Translate mkdir calls to place nodes'''
    def match(line):
        return re.match('^mkdir (.*)$', line)

    return [match(l).group(1) for l in lines if match(l)]


def extract_transitions(lines):
    '''Translate useradd calls to transition nodes'''
    def match(line):
        return re.match('^useradd .* pu_(.*)$', line)

    return [match(l).group(1) for l in lines if match(l)]


def extract_place_trans_edges(lines):
    '''Translate chown calls to 'place -> transition' edges'''
    def match(line):
        return re.match('^chown pu_(.*):pg_\S* (.*)$', line)

    return ['"'+match(l).group(2)+'"->"'+match(l).group(1)+'";'
            for l in lines if match(l)]


def extract_trans_place_edges(lines):
    '''Translate useradd and chown calls to 'transition -> place' edges'''
    def match(line):
        return re.match('^useradd -M -N -g (.*) -G (.*) pu_(.*)$', line)

    # Map a user to the groups it belongs to
    user2groups = {match(l).group(3): [match(l).group(1)[3:]] +
                   list(map(lambda x: x[3:], match(l).group(2).split(',')))
                   for l in lines if match(l)}
    # Invert the map: map a group to the users that belong to it
    group2users = {g: [u for u in user2groups if g in user2groups[u]] for
                   l in user2groups.values() for g in l}

    def match(line):
        return re.match('^chown (.*):pg_(\S*) (.*)$', line)

    # Map a directory to the users that belong in its group, but are not
    # the owner nor `whoami`
    dir2users = {match(l).group(3):
                 [u for u in group2users[match(l).group(2)]
                  if u != match(l).group(1)[3:] and u != '`whoami`']
                 for l in lines if match(l)}

    return ['"'+user+'"->"'+dir+'";'
            for dir in sorted(dir2users) if len(dir2users[dir]) > 0
            for user in sorted(dir2users[dir])]


def pmjq_viz():
    '''Entry point of the pmjq_viz executable

    Scan the script on the standard input to translate it to its Petri Net
    couterpart.'''
    lines = list(sys.stdin.readlines())
    places = extract_places(lines)
    transitions = extract_transitions(lines)
    place_trans_edges = extract_place_trans_edges(lines)
    trans_place_edges = extract_trans_place_edges(lines)
    print(DOT_TEMPLATE.format(places='\n'.join('"'+p+'";' for p in places),
                              transitions='\n'.join('"'+t+'";'
                                                    for t in transitions),
                              place_trans_edges='\n'.join(place_trans_edges),
                              trans_place_edges='\n'.join(trans_place_edges)))
