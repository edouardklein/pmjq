# In fish
# set -x GOPATH ~/src/go
# set -x PATH $PATH /usr/local/go/bin $GOPATH/bin
SHELL := /bin/bash  # We use pushd
DOC_STAGE_DIR=/tmp/pmjq  # mkdir $DOC_STAGE_DIR's parent, git clone pmjq in $DOC_STAGE_DIR, git checkout gh-pages

all: install test

pmjq: pmjq.go lint
	go build

install: pmjq
	sudo python3 setup.py install --old-and-unmanageable
	lnav -i lnav_pmjq.json
	sudo cp pmjq /usr/local/bin/
	pushd ui; lein figwheel ':once' min; popd
	sudo mkdir -p /usr/share/pmjq/js
	sudo cp ui/resources/public/index.html /usr/share/pmjq/
	sudo cp ui/resources/public/js/app.js /usr/share/pmjq/js/

SFF_FOLDER=/tmp/sff
serve:
	# Create or allocate a zone of your FS to be exposed to the outside
	mkdir -p $(SFF_FOLDER)/dl_zone  # Results will accumulate here
	mkdir -p $(SFF_FOLDER)/js
	# Populate it with the client code
	cp /usr/share/pmjq/index.html $(SFF_FOLDER)/
	cp /usr/share/pmjq/js/app.js $(SFF_FOLDER)/js/
	# Make sure one client can not see the result of another
	echo "Dir listing is forbidden" > $(SFF_FOLDER)/dl_zone/index.html
	# Create the endpoints of the transitions
	pmjq_mkendpoints  --exec=doc/smallest_transition.py '[smallest_transition]'
	# Actually start pmjq
	$$(pmjq_cmd --exec=doc/smallest_transition.py '[smallest_transition]') & \
    echo "$$!" > "$(SFF_FOLDER)/pmjq.pid"
	# Launch websocketd. It should be able to handle hundreds of not thousands of
	# simultaneous connections
	websocketd -staticdir=$(SFF_FOLDER) -port=8080 \
    pmjq_sff --exec=doc/smallest_transition.py \
    --dl-folder=$(SFF_FOLDER)/dl_zone \
    --dl-url=dl_zone '[smallest_transition]'

live:
	# Create the endpoints of the transitions
	pmjq_mkendpoints  --exec=doc/smallest_transition.py '[smallest_transition]'
	# Actually start pmjq
	$$(pmjq_cmd --exec=doc/smallest_transition.py '[smallest_transition]') & \
    echo "$$!" > /tmp/pmjq.pid
	# Launch websocketd.
	rm -rf /tmp/dl_zone
	mkdir -p /tmp/dl_zone
	websocketd -staticdir=ui/resources -port=8080 \
    pmjq_sff --exec=doc/smallest_transition.py \
    --dl-folder=/tmp/dl_zone \
    --dl-url=dl_zone '[smallest_transition]'& \
      echo "$$!" > /tmp/websocketd.pid
	pushd ui; lein figwheel; popd

.PHONY: install lint clean

docs:
	cd doc && pmjq_viz --exec smallest_transition.py '[smallest_transition]'\
		| dot -Tpng > smallest_transition.png
	cd doc && pmjq_viz --exec complete_transition.py --logs '[complete_transition]'\
		| dot -Tpng > complete_transition.png
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
	test_cases/func_command_gen.sh
	test_cases/func_error.sh
	test_cases/func_log.sh
	test_cases/func_regex_pattern.sh
	test_cases/bug_sshfs.sh
	test_cases/bug_spacename.sh
	test_cases/bug_cartesianproduct.sh
	test_cases/bug_quote.sh
	test_cases/func_sff.sh
	test_cases/bug_trailing_slash.sh

test: test_pmjq

lint:
	golint pmjq.go

clean:
	rm -rf pmjq build/ dist/ *.egg-info test_dir*/ doc/smallest_transition.png doc/complete_transition.png
	-kill -9 $$(cat ${SFF_FOLDER}/pmjq.pid)
	-kill -9 $$(cat /tmp/pmjq.pid)
	-kill -9 $$(cat /tmp/websocketd.pid)
	make -C doc clean
	pushd ui; lein clean; popd
	-rm -r pmjqtools/__pycache__
	-rm -r doc/_build

# With entr:
#  find . -regextype posix-extended -type f -iregex '(.*\.(sh|py|go)|\./Makefile)' | entr -rc make install test | grcat conf.gcc
