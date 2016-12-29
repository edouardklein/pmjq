#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp

rm -rf ${PLAYGROUND}/input
rm -rf ${PLAYGROUND}/output

mkdir -p ${PLAYGROUND}/input
mkdir -p ${PLAYGROUND}/output

echo -n ok > ${PLAYGROUND}/input/aaa_OK.txt
echo -n ok > ${PLAYGROUND}/input/bbb_OK.txt
echo -n ok > ${PLAYGROUND}/input/ccc_OK.txt
echo -n ok > ${PLAYGROUND}/input/ddd_OK.txt
echo -n itsatrap > ${PLAYGROUND}/input/eee_OK.mp3
echo -n notok > ${PLAYGROUND}/input/notok.txt
echo -n badnamepattern > ${PLAYGROUND}/input/lkfqsdmfjsd

cd "$(dirname "$0")"
../pmjq --quit-when-empty \
        ${PLAYGROUND}'/input/(...)_OK.txt' \
        "cat" \
        ${PLAYGROUND}'/output/$1_$(cat)_PROCESSED.txt' \
        &> ${PLAYGROUND}/pmjq.log

for prefix in aaa bbb ccc ddd
do
    if [ ! -f ${PLAYGROUND}/output/${prefix}_ok_PROCESSED.txt ]; then
        echo "Output file ${prefix}_ok_PROCESSED.txt does not exist "
        exit 1
    fi
done

for file in eee_OK.mp3 notok.txt lkfqsdmfjsd
do
    if [ ! -f ${PLAYGROUND}/input/${file} ]; then
        echo "Non matching input file ${file} was deleted"
        exit 1
    fi
done
