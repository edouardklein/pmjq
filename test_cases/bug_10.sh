#!/usr/bin/env bash
# A (yet of unknown source) strange bug appears on my machine when I try to
# md5sum a large-ish number of files.
# The bug does not appear when the number is small (~ 100)
# It randomly appears when the number is around 2500
# It appears every time with 10 000 files
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp

mkdir -p ${PLAYGROUND}/input
mkdir -p ${PLAYGROUND}/output

rm -rf ${PLAYGROUND}/input/*
rm -rf ${PLAYGROUND}/output/*

for file in $(seq 10000)
do
    echo $file > ${PLAYGROUND}/input/$file.txt
done

# Launch pmjq in the background
pmjq --quit-when-empty ${PLAYGROUND}/input "md5sum" ${PLAYGROUND}/output
