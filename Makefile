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
	rm -rf test_dir
	mkdir test_dir
	cd test_dir && \
		grep -v -E '^#' < ../test.txt | pmjq_interactive  && pmjq_viz < setup.sh | dot -T pdf > test.pdf

test:
	rm -rf test_dir
	mkdir test_dir
	cd test_dir && \
		../pmjqtools/test.py && \
		coverage report | grep pmjq

paper: paper/main.pdf install

paper/main.pdf:
	make -C paper
