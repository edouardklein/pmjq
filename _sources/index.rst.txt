.. PMJQ Tools documentation master file, created by
   sphinx-quickstart on Wed Mar 30 11:52:10 2016.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.

#######################################
PMJQ
#######################################

PMJQ is a daemon that watches one or more directories for new files, process them using a user-provided command, and dumps the results in one or more other directories.

Chaining multiple instances allows one to build a whole data processing flowchart made up of individual commands.

Sharing the filesystem between mutliple machines allows one to share the load transparently.

The daemon is written in Go and can be statically compiled.

The supporting tools are written in Python.

A more complete documentation has yet to be written.


***************************
Installation from sources
***************************

The only hard dependency for building the package is the  `Go programming language <https://golang.org/doc/install>`_. Optional dependencies include:

* `lnav <http://lnav.org/>`_
* `sshfs <https://github.com/libfuse/sshfs>`_
* `golint <https://godoc.org/golang.org/x/lint/golint>`_
* Python 3
* `entr <http://entrproject.org/>`_
* `websocketd <http://websocketd.com/>`_

Clone the repo and run ``make install`` in it. To only install the tool and none of its supporting components, run ``go install`` instead.

**************************
Basic Usage
**************************


.. program-output:: pmjq --help


*************************
Domain Specific Language
*************************

.. automodule:: pmjqtools.dsl
    :members:

*************************
Helper tools
*************************

.. automodule:: pmjqtools.tools
   :members:

**************************
User Interface
**************************

Dataflow designers should refer to the DSL documentation (:py:mod:`pmjqtools.dsl`).
This section is about the end-users of the dataflow: the people who are going
to provide the input data and use the output data.

File system
============

The basic UI is to mount the input and output directories somewhere in the user's
namespace. In the field, we used `sshfs` and `samba` with great success.

The users then simply drop their files in the input directory or directories, and
wait for the end-product to appear in the output directory or directories.

Web UI
========

.. automodule:: pmjqtools.pmjq_sff
   :members:

