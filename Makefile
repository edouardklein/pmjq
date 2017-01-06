DOC_STAGE_DIR=~/Bureau/pmjq_docs/

all: install test

pmjq: pmjq.go lint
	go build --ldflags '-extldflags "-static"'

install: pmjq
	sudo python3 setup.py install --old-and-unmanageable
	lnav -i keyvalue.json
	sudo cp pmjq /usr/local/bin/

.PHONY: install lint clean

docs:
	make -C paper whole_pipeline.png
	cd doc && \
    ./draw_FSM.sh | dot -T png > FSM.png
	make -C doc html

upload_docs: docs
	cd $(DOC_STAGE_DIR) && \
		git rm -r .
	cp -r doc/_build/html/* $(DOC_STAGE_DIR)
	cd $(DOC_STAGE_DIR) && \
		touch .nojekyll && \
		git add . && \
		git commit -m "Doc update" && \
		git push origin gh-pages

test_pmjq: pmjq
	test_cases/bug_10.sh
	test_cases/func_error.sh
	test_cases/func_log.sh
	test_cases/func_regex_pattern.sh
	test_cases/bug_sshfs.sh

test_pmjqtools:
	test_cases/func_command_gen.sh

test: test_pmjq #test_pmjqtools test_pmjq

lint:
	golint pmjq.go

clean:
	rm -rf pmjq build/ dist/ *.egg-info test_dir*/

# With entr:
#  find . -regextype posix-extended -type f -iregex '(.*\.(sh|hy|py|go)|\./Makefile)' | entr -rc make install test | grcat conf.gcc
