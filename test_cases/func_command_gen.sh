#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp

for file in $(dirname $0)/*.hy
do
    # Is the generated command in the corresponding test file
    gen_command=$(hy ${file})
    grep -x -F "${gen_command}" $(dirname ${file})/$(basename ${file} .hy).sh
done
