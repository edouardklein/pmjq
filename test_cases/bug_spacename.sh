#!/usr/bin/env bash
# A space (or any other meaningful) in a file may throw pmjq off the rails
# This test make sure it does not.
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp
MD5_CMD=md5sum

rm -rf ${PLAYGROUND}/input
rm -rf ${PLAYGROUND}/output
rm -rf ${PLAYGROUND}/error

mkdir -p ${PLAYGROUND}/input
mkdir -p ${PLAYGROUND}/output
mkdir -p ${PLAYGROUND}/error

echo "funny file name" > ${PLAYGROUND}'/input/space dot.dollar$lol'

cd "$(dirname "$0")"
pmjq --quit-when-empty --input=${PLAYGROUND}/input/'.*' ${MD5_CMD} --output=${PLAYGROUND}/output/ &> ${PLAYGROUND}/pmjq.log

ls ${PLAYGROUND}/output/ | wc -l | grep -x 1

if [ -f ${PLAYGROUND}/input/* ]; then
    echo "Not all files in the input dir have been processed"
    exit 1
fi

