all: test docs paper

docs:
	make -C doc html

paper: paper/main.pdf

paper/main.pdf:
	make -C paper
