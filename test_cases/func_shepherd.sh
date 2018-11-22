#!/usr/bin/env bash
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp

rm -rf ${PLAYGROUND}/minimal_service.scm
rm -rf ${PLAYGROUND}/complete_service.scm

pmjq_herd --exec=doc/smallest_transition.py '[smallest_transition]' > ${PLAYGROUND}/minimal_service.scm
diff test_cases/func_shepherd_minimal.scm ${PLAYGROUND}/minimal_service.scm

pmjq_herd --exec=doc/complete_transition.py '[complete_transition]' > ${PLAYGROUND}/complete_service.scm
diff test_cases/func_shepherd_complete.scm ${PLAYGROUND}/complete_service.scm
