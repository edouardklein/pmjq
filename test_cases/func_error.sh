#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp

mkdir -p ${PLAYGROUND}/input
mkdir -p ${PLAYGROUND}/output
mkdir -p ${PLAYGROUND}/error

rm -rf ${PLAYGROUND}/input/*
rm -rf ${PLAYGROUND}/output/*
rm -rf ${PLAYGROUND}/error/*

echo OK > ${PLAYGROUND}/input/OK.txt
echo error > ${PLAYGROUND}/input/error.txt

cd "$(dirname "$0")"
../pmjq --quit-when-empty --error-dir=${PLAYGROUND}/error ${PLAYGROUND}/input "grep -v error" ${PLAYGROUND}/output &> ${PLAYGROUND}/pmjq.log

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


