#!/usr/bin/env bash
# Quoting and unquoting is always cumbersome. Here is the trajectory
# of the string that define e.g. a command in a transition:
# 1. First it is defined in Python by the dataflow designer
# 2. Then it is used in Python by the pmjqtools package get a pmjq shell command
# 3. Then this pmjq shell command is run in a shell
# 4. Then pmjq itself handles the command (a template), expanding it to get
#    the actual command to run in order to process the data
# 5. Finally the command is ran
#
# It looks complicated, but there actually exists a foolproof
# and simple method: quote everything that should not be interpreted by the
# shell at step 3.
# This means quoting everything unless you want e.g. a variable (such as
# $PLAYGROUND in our test scripts) to be expanded before pmjq sees the string
#
# Anytime we see a problem with quoting we'll add a minimum working
# example of the bug to this test case.
set -e
set -u
set -x
set -o pipefail



PLAYGROUND=/tmp
MD5_CMD=md5sum

rm -rf ${PLAYGROUND}/input
rm -rf ${PLAYGROUND}/output


rm -rf ${PLAYGROUND}/\'input
rm -rf ${PLAYGROUND}/\'output


echo "from shlex import quote as q
t = [{
    'inputs': [q('"${PLAYGROUND}/input/"(?P<id>.*).xml)')],
    'outputs': [q('"${PLAYGROUND}/output/"{{.Input 0}}')],
    'cmd': q('"${MD5_CMD}"')
}]
" > ${PLAYGROUND}/sc.py

pmjq_mkendpoints --exec=${PLAYGROUND}/sc.py t

if [ ! -e /tmp/input/ ] || [ ! -e /tmp/output/ ]; then
    echo "Folders weren't created as they should have!"
fi
