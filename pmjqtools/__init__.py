'''The pmjqtools module provides supporting tools for the PMJQ distributed
computing framework.

Basic principle
===================

PMJQ is a daemon that monitors one or several input directories. It launches a
process everytime there is data to process in the input directories. The
process writes its output in one or more output directories.

Multiple instances can be chained (one reading from an output directory of an
onther), to compose an arbitrarily complex processing network.

By distributing the underlying file system, one can distribute the computation
across several nodes.

The reference document for PMJQ is the paper. It
gives an overview of PMJQ and its tooling, and demonstrate that PMJQ maps to a
powerful mathematical model of distributed computing: the Petri Nets.

PMJQ concepts
==============

PMJQ can be used to build complex processing pipelines, with branching and
merging.

A `pipeline` is composed of multiple `invocations`. An invocation is a line in
a script that calls pmjq, which will read its input from some directories and
write its output in some others.

Note that there can exist multiple `instances` of this invocation, on multile
machines, to distribute the computation among the nodes of a cluster.

Invocations can be chained (see the paper).

Where to put that ?
=====================

The paper can be build by issuing ``make paper`` in the root directory of the
project. The paper's figures are not fixed, they are built with the tooling
provided by
this package.

'''
from .tools import *
