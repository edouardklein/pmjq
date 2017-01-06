#!/usr/bin/env bash
# Trying to process 10.000 files exposes a race condition in a previous version of PMJQ
# What happens most of the time:
#  dirLister             locker, worker, etc.
#      |
#    List---------------------+
#      |                      |
#      |                    Lock
#      |                      |
#      |                    Process
#      |                      |
#      |                    Remove
#      |                      |
#      |                    Unlock
#      |
#     List---------------------------------+
#                                          |
#                                        Lock
#                                          |
#                                        Process
#                                          |
#                                        Remove
#                                          |
#                                        ...
# What happens sometimes and makes everything crash:
#   dirLister              locker, worker, etc.
#       |
#     List-------------------------|
#       |                          |
#       |                        Lock
#       |                          |
#       |                        Process
#       |                          |
#      List----------------------------------|
#       |                          |         |
#       |                        Remove      |
#       |                          |         |
#       |                        Unlock      |
#       |                          |         |
#       |                          |       Lock
#       |                          |         |
#       |                          |       Process /!\ Files don't exist anymore
#
# One solution is to forbid the locking of non existant files
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

NB=10000  # Bug always manifested itself for $NB >= 10000, but keep in mind its appearance is stochastic

for file in $(seq ${NB})
do
    echo $file > ${PLAYGROUND}/input/$file.txt
done

cd "$(dirname "$0")"
pmjq --quit-when-empty --input=${PLAYGROUND}/input/'.*' ${MD5_CMD} --output=${PLAYGROUND}/output/ &> ${PLAYGROUND}/pmjq.log

ls ${PLAYGROUND}/output/ | wc -l | grep -x ${NB}

if [ -f ${PLAYGROUND}/input/* ]; then
    echo "Not all files in the input dir have been processed"
    exit 1
fi
