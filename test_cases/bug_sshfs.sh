#!/usr/bin/env bash
# The lockfile library we previously used, liblockfile, does
# not work on non-POSIX compliant filesystems
# which means it does not work on SSHFS (shame on you, SSHFS!)
# therefore we had to build our own, more robust but less efficient,
# lockfile code.
# This test is here to ensure it works on SSHFS
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp
MD5_CMD=md5sum


if ! mount | grep ${PLAYGROUND}/dir2; then
    rm -rf ${PLAYGROUND}/dir1
    rm -rf ${PLAYGROUND}/dir2
    mkdir -p ${PLAYGROUND}/dir1
    mkdir -p ${PLAYGROUND}/dir2
    sshfs $(whoami)@127.0.0.1:${PLAYGROUND}/dir1 ${PLAYGROUND}/dir2
fi
PLAYGROUND=${PLAYGROUND}/dir2

rm -rf ${PLAYGROUND}/input
rm -rf ${PLAYGROUND}/output
rm -rf ${PLAYGROUND}/error

mkdir -p ${PLAYGROUND}/input
mkdir -p ${PLAYGROUND}/output
mkdir -p ${PLAYGROUND}/error

NB=10

for file in $(seq ${NB})
do
    echo $file > ${PLAYGROUND}/input/$file.txt
done

cd "$(dirname "$0")"
# Usage of timeout because the bug manifests itself as the lock
# never getting acquired and thus as the files never getting processed
# which lets pmjq run forever
timeout 10 pmjq --quit-when-empty --input=${PLAYGROUND}/input/'.*' ${MD5_CMD} --output=${PLAYGROUND}/output/ &> ${PLAYGROUND}/pmjq.log

ls ${PLAYGROUND}/output/ | wc -l | grep -x ${NB}

if [ -f ${PLAYGROUND}/input/* ]; then
    echo "Not all files in the input dir have been processed"
    exit 1
fi

sudo umount ${PLAYGROUND}
