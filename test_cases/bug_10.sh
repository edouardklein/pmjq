#!/usr/bin/env bash
# If the locker does not check if a file exist before it gives it to the spawner,
# sometimes we have a bad time (we open a file that does not exist anymore)
# The bug does not appear when the number is small (~ 100)
# It randomly appears when the number is around 2500
# It appears every time with 10 000 files
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

for file in $(seq 10000)
do
    echo $file > ${PLAYGROUND}/input/$file.txt
done

cd "$(dirname "$0")"
../pmjq --quit-when-empty ${PLAYGROUND}'/input/.*' ${MD5_CMD} ${PLAYGROUND}'/output/$0' &> ${PLAYGROUND}/pmjq.log

if [ -f ${PLAYGROUND}/error/* ]; then
    echo "There were errors but there should not have been any"
    exit 1
fi

if [ -f ${PLAYGROUND}/input/* ]; then
    echo "Not all files in the input dir have been processed"
    exit 1
fi
