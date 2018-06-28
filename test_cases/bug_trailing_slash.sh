#!/usr/bin/env bash
# A bug was discovered in pmjq_sff where the log, input or output paths could not end
# with a trailing slash, whereas the doc and the Golang code say they can, in which
# case:
# - the '.*' at the end is implicit for input paths
# - the '{{Input 0}}' at the end is implicit for output and log paths
# We make sure here that the incriminated function returns correct values.
# We unit test instead of integration test because an integration test
# would be illegible.

set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp/sff
rm -rf ${PLAYGROUND}
mkdir -p ${PLAYGROUND}

cat >"${PLAYGROUND}/transition.py" <<EOF
smallest_transition = {'stdin': '/tmp/input/',
                       'stdout': '/tmp/output/',
                       'cmd': 'sha256sum'}
EOF

cat >"${PLAYGROUND}/unit_test_find_leaves.py" <<EOF
from pmjqtools.pmjq_sff import find_leaves
from pmjqtools.dsl import run_on_transitions_from_cli
import sys
PLAYGROUND=sys.argv[1]
args = {"--exec": PLAYGROUND+'/transition.py', "<eval>": '[smallest_transition]'}
answer = run_on_transitions_from_cli(args, find_leaves)
assert answer == ({'/tmp/input'}, {'/tmp/output'}, set()), \
"find_leaves does not yield the expected answer, it yields: "+str(answer)
EOF

python3 ${PLAYGROUND}/unit_test_find_leaves.py $PLAYGROUND
