all: test docs paper

docs:
	make -C doc html

test:
	rm -rf test_dir
	mkdir test_dir
	cd test_dir; ../pmjqtools/test.py

paper: paper/main.pdf

paper/main.pdf:
	make -C paper
