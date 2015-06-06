install: doc
	install -bC pmjq /usr/local/bin/
	install -bC doc/pmjq.1 /usr/local/share/man/man1
	lnav -i keyvalue.json

pmjq: pmjq.go
	go build

doc: doc/pmjq.1 doc/pmjq.1.html

view_doc: doc/pmjq.1.ronn
	ronn doc/pmjq.1.ronn --man

doc/pmjq.1: doc/pmjq.1.ronn
	ronn doc/pmjq.1.ronn 

doc/pmjq.1.html: doc/pmjq.1.ronn
	ronn doc/pmjq.1.ronn --html

README.md: doc/pmjq.1.ronn
	sed 's/</`/g' < doc/pmjq.1.ronn | sed 's/>/`/g'> README.md

clean:
	rm doc/pmjq.1 doc/pmjq.1.html

