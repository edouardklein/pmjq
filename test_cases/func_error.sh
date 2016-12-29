#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp

rm -rf ${PLAYGROUND}/input
rm -rf ${PLAYGROUND}/output
rm -rf ${PLAYGROUND}/error

mkdir -p ${PLAYGROUND}/input
mkdir -p ${PLAYGROUND}/output
mkdir -p ${PLAYGROUND}/error

echo OK > ${PLAYGROUND}/input/OK.txt
echo error > ${PLAYGROUND}/input/error.txt

cd "$(dirname "$0")"
../pmjq --quit-when-empty --stderr=${PLAYGROUND}'/error/$0' ${PLAYGROUND}'/input/.*' "grep -v error" ${PLAYGROUND}'/output/$0' &> ${PLAYGROUND}/pmjq.log

if [ ! -f ${PLAYGROUND}/output/OK.txt ]; then
    echo "OK file was not processed"
    exit 1
fi

if [ ! -f ${PLAYGROUND}/error/error.txt ]; then
    echo "Error-triggering file was not put in error dir"
    exit 1
fi

if [ -f ${PLAYGROUND}/input/* ]; then
    echo "Not all files in the input dir have been processed"
    exit 1
fi


