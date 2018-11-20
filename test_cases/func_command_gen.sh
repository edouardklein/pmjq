#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp


files=(
    test_cases/bug_10.py
    test_cases/func_error.py
    test_cases/func_log.py
)

for file in "${files[@]}"
do
    echo Hello $file
    # Is the generated command in the corresponding test file
    gen_command=$(pmjq_cmd --exec ${file} 'transitions')
    grep -x -F "${gen_command}" $(dirname ${file})/$(basename ${file} .py).sh
done
