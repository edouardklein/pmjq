#!/usr/bin/env bash
# pmjq_viz would fail at unquoting the transitions in some cases.
# This test makes sure it doesn't
set -e
set -u
set -x
set -o pipefail

PLAYGROUND=/tmp

pmjq_viz --exec ./test_cases/bug_quote_transitions.py transitions > ${PLAYGROUND}/result.dot

[ ! "`grep "'" ${PLAYGROUND}/result.dot`" ]