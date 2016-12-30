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
pmjq --quit-when-empty --input=${PLAYGROUND}/input0/'(?P<id>...)_(?P<suffix>.*)\.txt' --input=${PLAYGROUND}/input1/'(?P<prefix>.*)_(?P<id>...)\.sum' cat --output=${PLAYGROUND}/outputA/'{{.namedmatch.id}}_{{.namedmatch.prefix}}_{{.namedmatch.suffix}}.txt' --output=${PLAYGROUND}/outputB/'{{call .timestamp}}_{{.invariant}}' --stderr=${PLAYGROUND}/log/'{{.invariant}}.log' --error=${PLAYGROUND}/error0/'{{index .match 0 0}}' --error=${PLAYGROUND}/error1/'{{index .match 1 0}}' &> ${PLAYGROUND}/pmjq.log

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
