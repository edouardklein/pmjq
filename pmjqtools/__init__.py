'''The pmjqtools module provides supporting tools for the ``pmjq`` distributed
computing framework.

Basic principle
===================

``pmjq`` is a daemon that monitors one or several input directories. It launches a
process everytime there is data to process in the input directories. The
process writes its output in one or more output directories.

Multiple instances can be chained (one reading from an output directory of an
onther), to compose an arbitrarily complex processing network.

By distributing the underlying file system, one can distribute the computation
across several nodes.

The reference document for ``pmjq`` is the `paper
<https://github.com/edouardklein/pmjq/raw/master/paper/main.pdf>`_. It
gives an overview of ``pmjq`` and its tooling, and demonstrate that ``pmjq`` maps to a
powerful mathematical model of distributed computing: the Petri Nets.

``pmjq`` concepts
==============

``pmjq`` can be used to build complex processing pipelines, with branching and
merging.

A `pipeline` is composed of multiple `invocations`. An invocation is a line in
a script that calls ``pmjq``, making it read its input from some directories and
write its output in some others.

Note that there can exist multiple `instances` of this invocation, on multile
machines, to distribute the computation among the nodes of a cluster.

Invocations can be chained (see the paper for the details) in order to create
complete data processing workflows, such as this one for video processing:

.. image:: ../paper/whole_pipeline.png

Installation
=====================

Tools
------
The tooling can be installed with::

    make install

Note that this only needs to be installed on the designer's machine. In order to run a ``pmjq`` pipeline, one only needs the pmjq executable which, being written in Golang, is statically linked and can therefore be copied to the computation nodes directly.


Paper
------

The paper can be build by issuing ``make paper`` in the root directory of the
project. The paper's figures are not fixed, they are built with the tooling
provided by this package. Therefore one has to run ``make install`` beforehand

'''
from .tools import *
