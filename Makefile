DOC_STAGE_DIR=~/Bureau/pmjq_docs/

all: test docs paper

install:
	python3 setup.py install
	go build
	cp pmjq /usr/local/bin/

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

test_pmjq:
	rm -rf md5ed sha1ed md5_pool sha1_pool input output test_output.txt
	mkdir md5ed sha1ed md5_pool sha1_pool input output
	./test.sh
	echo a > input/a
	((echo a | md5sum) && (echo a | sha1sum)) > test_output.txt
	while [ ! -f output/a ]
	do
		echo Waiting for pmjq to do its job
		sleep 2
	done
	diff output/a test_output.txt
	killall -q pmjq || true

test:
	rm -rf test_dir
	mkdir test_dir
	cd test_dir && \
		../pmjqtools/test.py && \
		coverage report | grep pmjq

paper: paper/main.pdf install

paper/main.pdf:
	make -C paper
