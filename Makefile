all: test docs paper

install:
	python3 setup.py install

docs:
	make -C doc html

test:
	rm -rf test_dir
	mkdir test_dir
	cd test_dir && \
		../pmjqtools/test.py && \
		coverage report | grep pmjq

paper: paper/main.pdf install

paper/main.pdf:
	make -C paper
