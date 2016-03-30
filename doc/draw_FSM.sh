#!/bin/bash
echo "digraph G{"
# Edge pass
grep '{Src: .*, Dst: .*, Name: .*' < ../pmjq.go \
    | sed 's/{Src: \[\]string{//' \
    | sed 's/}, Dst: / -> /'\
    | sed 's/, Name: /[label=/'\
    | sed 's/},/];/'
# Node pass
grep 'func(e \*fsm.Event) {.*}' < ../pmjq.go \
    | sed 's/: *func(e \*fsm.Event)//' \
    | sed 's/ { / [label="/'\
    | sed 's/ },/"];/'
echo "}"
