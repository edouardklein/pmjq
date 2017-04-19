#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp
! read -d '' EXAMPLE_COMMAND <<"EOF"
python3 -c "import sys
sys.stderr.write('Hello stderr')
data1 = open(sys.argv[1]).read()
data2 = open(sys.argv[2]).read()
if data1 != data2:
    sys.exit(1)
open(sys.argv[3], 'w').write('ok')
open(sys.argv[4], 'w').write('ok')"
EOF


rm -rf ${PLAYGROUND}/input0
rm -rf ${PLAYGROUND}/input1
rm -rf ${PLAYGROUND}/outputA
rm -rf ${PLAYGROUND}/outputB
rm -rf ${PLAYGROUND}/error0
rm -rf ${PLAYGROUND}/error1
rm -rf ${PLAYGROUND}/log

mkdir -p ${PLAYGROUND}/input0
mkdir -p ${PLAYGROUND}/input1
mkdir -p ${PLAYGROUND}/outputA
mkdir -p ${PLAYGROUND}/outputB
mkdir -p ${PLAYGROUND}/error0
mkdir -p ${PLAYGROUND}/error1
mkdir -p ${PLAYGROUND}/log

echo -n oka > ${PLAYGROUND}/input0/aaa_OK.txt
echo -n okb > ${PLAYGROUND}/input0/bbb_OK.txt
echo -n okc > ${PLAYGROUND}/input0/ccc_OK.txt
echo -n okd > ${PLAYGROUND}/input0/ddd_OK.txt
echo -n error0 > ${PLAYGROUND}/input0/err_foo.txt
echo -n badnamepattern > ${PLAYGROUND}/input0/foobar


echo -n oka > ${PLAYGROUND}/input1/Foo_aaa.txt
echo -n okb > ${PLAYGROUND}/input1/Foo_bbb.txt
echo -n okc > ${PLAYGROUND}/input1/Foo_ccc.txt
echo -n okd > ${PLAYGROUND}/input1/Foo_ddd.txt
echo -n error1 > ${PLAYGROUND}/input1/bar_err.txt
echo -n badnamepattern > ${PLAYGROUND}/input1/barfoo



cd "$(dirname "$0")"
pmjq \
    --quit-when-empty\
    --input=${PLAYGROUND}/input0/'(?P<id>...)_(?P<suffix>.*)\.txt'\
    --input=${PLAYGROUND}/input1/'(?P<prefix>.*)_(?P<id>...)\.txt'\
    --invariant='$id'\
    "${EXAMPLE_COMMAND} ${PLAYGROUND}/input0/'{{.Input 0}}' ${PLAYGROUND}/input1/'{{.Input 1}}' ${PLAYGROUND}/outputA/'{{.NamedMatches.id}}_{{.NamedMatches.prefix}}_{{.NamedMatches.suffix}}.txt' ${PLAYGROUND}/outputB/'{{.Invariant}}.txt'"\
    --output=${PLAYGROUND}/outputA/'{{.NamedMatches.id}}_{{.NamedMatches.prefix}}_{{.NamedMatches.suffix}}.txt'\
    --output=${PLAYGROUND}/outputB/'{{.Invariant}}.txt'\
    --stderr=${PLAYGROUND}/log/'{{.Invariant}}.log'\
    --error=${PLAYGROUND}/error0/'{{.Input 0}}'\
    --error=${PLAYGROUND}/error1/'{{.Input 1}}' &> ${PLAYGROUND}/pmjq.log

for id in aaa bbb ccc ddd
do
    if [ ! -f ${PLAYGROUND}/outputA/${id}_Foo_OK.txt ]; then
        echo "Output file outputA/${id}_Foo_OK.txt does not exist"
        exit 1
    fi
    if [ ! -f ${PLAYGROUND}/outputB/${id}.txt ]; then
        echo "Output file outputB/${id}.txt does not exist"
        exit 1
    fi
    if [ ! -f ${PLAYGROUND}/log/${id}.log ]; then
        echo "Output file log/${id}.log does not exist"
        exit 1
    fi
done

if [ ! -f ${PLAYGROUND}/input0/foobar ]; then
    echo "Non matching input file input0/foobar was deleted"
    exit 1
fi

if [ ! -f ${PLAYGROUND}/input1/barfoo ]; then
    echo "Non matching input file input1/barfoo was deleted"
    exit 1
fi

if [ ! -f ${PLAYGROUND}/error0/err_foo.txt ]; then
    echo "Error file error0/err_foo.txt was not moved"
    exit 1
fi

if [ ! -f ${PLAYGROUND}/error1/bar_err.txt ]; then
    echo "Error file error1/bar_err.txt was not moved"
    exit 1
fi

if [ ! -f ${PLAYGROUND}/log/err.log ]; then
    echo "Log file log/err.log does not exist"
    exit 1
fi

if ls ${PLAYGROUND}/*/*.lock 1> /dev/null 2>&1; then
    echo "At least one lock file remains (All should be deleted)."
    exit 1
fi
