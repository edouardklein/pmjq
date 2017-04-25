#!/usr/bin/env bash
# When there are too many files in a multiple input transition, our naive
# cartesian product implementation tries to allocate a huge array, which leads to
# an out of memory error.
# This test makes sure we use a smarter implementation instead.
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp
MD5_CMD=md5sum

rm -rf ${PLAYGROUND}/input0
rm -rf ${PLAYGROUND}/input1
rm -rf ${PLAYGROUND}/output
rm -rf ${PLAYGROUND}/error

mkdir -p ${PLAYGROUND}/input0
mkdir -p ${PLAYGROUND}/input1
mkdir -p ${PLAYGROUND}/output
mkdir -p ${PLAYGROUND}/error

# http://www.heatware.net/linux-unix/create-large-many-number-files-thousands-millions/
dd if=/dev/zero of=${PLAYGROUND}/masterfile bs=1 count=10000
split -b 1 -a 6 ${PLAYGROUND}/masterfile ${PLAYGROUND}/input0/
split -b 1 -a 6 ${PLAYGROUND}/masterfile ${PLAYGROUND}/input1/
rm ${PLAYGROUND}/masterfile

cd "$(dirname "$0")"
pmjq \
    --quit-when-empty\
    --input=${PLAYGROUND}/input0/\
    --input=${PLAYGROUND}/input1/\
    --invariant='$0'\
    "cat ${PLAYGROUND}/input0/'{{.Input 0}}' ${PLAYGROUND}/input0/'{{.Input 1}}'"\
    --output=${PLAYGROUND}/output/'{{.Input 0}}.txt'\
    --stderr=${PLAYGROUND}/log/'{{.Invariant}}.log'\
    --error=${PLAYGROUND}/error0/'{{.Input 0}}'\
    --error=${PLAYGROUND}/error1/'{{.Input 1}}' &> ${PLAYGROUND}/pmjq.log


ls ${PLAYGROUND}/output/ | wc -l | grep -x 10000

if [ -f ${PLAYGROUND}/input/* ]; then
    echo "Not all files in the input dir have been processed"
    exit 1
fi

