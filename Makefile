README.md: doc/pmjq.1.ronn
	sed 's/</`/g' < doc/pmjq.1.ronn | sed 's/>/`/g'> README.md

clean:
	rm doc/pmjq.1 doc/pmjq.1.html

