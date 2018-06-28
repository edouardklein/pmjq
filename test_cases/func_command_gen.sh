#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp

for file in $(dirname $0)/*.py
do
    # Is the generated command in the corresponding test file
    gen_command=$(pmjq_cmd --exec ${file} 'transitions')
    grep -x -F "${gen_command}" $(dirname ${file})/$(basename ${file} .py).sh
done
