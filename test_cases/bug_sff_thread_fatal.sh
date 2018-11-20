#!/usr/bin/env bash
# The following script should fail if threads that are started
# are not stopped
set -e
set -u
set -x
set -o pipefail


PLAYGROUND=/tmp/psff
MD5_CMD=md5sum
FILES=${PLAYGROUND}/files
rm -rf ${PLAYGROUND}/input
rm -rf ${PLAYGROUND}/output
rm -rf ${PLAYGROUND}/error
rm -rf ${PLAYGROUND}/log
rm -rf ${PLAYGROUND}/files
rm -f ${PLAYGROUND}/pmjq.log

mkdir -p ${PLAYGROUND}/input
mkdir -p ${PLAYGROUND}/output
mkdir -p ${PLAYGROUND}/error
mkdir -p ${PLAYGROUND}/log
mkdir -p ${PLAYGROUND}/files

# Create mock input

echo "name:${PLAYGROUND}/input:file.txt" > ${FILES}/file.txt
echo "data:${PLAYGROUND}/input:"$(echo Hello | base64) >> ${FILES}/file.txt
MD5_SUM=09f7e02f1290be211da707a266f153b3

cat> "${FILES}/trans.py" <<"EOF"
from shlex import quote as q;
T = [{"id": "test",
        "error": "errors/input/",
        "stderr": "logs/input/"+q("{{.NamedMatches.id}}.log"),
        "inputs": [q("input/(?P<id>.*).*")],
        "outputs": [q("output/{{.Input 0}}")],
        "cmd": "md5sum",
        "log": "./pmjq.log",
    },]
EOF

pmjq --input=${PLAYGROUND}/input/'.*' ${MD5_CMD} --output=${PLAYGROUND}/output/ &> ${PLAYGROUND}/pmjq.log&
PMJQ_PID=$!
echo "If this script fails, launch kill -9 ${PMJQ_PID}"


i=0
while [ $i -lt 50 ]; do
    pmjq_sff --exec="${FILES}/trans.py" --dl-folder=${FILES} --dl-url="" --root=${PLAYGROUND} T < ${FILES}/file.txt > ${FILES}/output.txt
    i=$((i+1))
done



kill -9 ${PMJQ_PID}
