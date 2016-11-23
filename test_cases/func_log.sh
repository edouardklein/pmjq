#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp
! read -d '' EXAMPLE_COMMAND <<"EOF"
python3 -c "import sys; \
sys.stderr.write('Hello stderr');\
sys.stdout.write(sys.stdin.read())"
EOF

mkdir -p ${PLAYGROUND}/input
mkdir -p ${PLAYGROUND}/output
mkdir -p ${PLAYGROUND}/error
mkdir -p ${PLAYGROUND}/log

rm -rf ${PLAYGROUND}/input/*
rm -rf ${PLAYGROUND}/output/*
rm -rf ${PLAYGROUND}/error/*
rm -rf ${PLAYGROUND}/log/*

echo this file is a token whose content doesnt matter > ${PLAYGROUND}/input/stderr.txt

cd "$(dirname "$0")"
../pmjq --quit-when-empty --log-dir=${PLAYGROUND}/log --error-dir=${PLAYGROUND}/error ${PLAYGROUND}/input "${EXAMPLE_COMMAND}" ${PLAYGROUND}/output &> ${PLAYGROUND}/pmjq.log

if [ ! -f ${PLAYGROUND}/output/stderr.txt ]; then
    echo "File was not processed"
    exit 1
fi

if [ -f ${PLAYGROUND}/error/stderr.txt ]; then
    echo "There was an error"
    exit 1
fi

if [ -f ${PLAYGROUND}/input/* ]; then
    echo "Not all files in the input dir have been processed"
    exit 1
fi

grep "Hello stderr" ${PLAYGROUND}/log/stderr.txt

