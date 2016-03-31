'''This module regroups the different templates for the scripts created
by the tools.'''

SETUP_TEMPLATE = '''#!/usr/bin/env sh
{groupadd}

{usermod}

{useradd}
{mkdir}
'''

CLEANUP_TEMPLATE = '''#!/usr/bin/env sh
{groupdel}

{userdel}

{rm}
'''

MKDIR_TEMPLATE = '''mkdir {directory}
chmod 510 {directory}
chown {user}:{group} {directory}
'''

PMJQ_FILTER_TEMPLATE = '''sudo -u {user} pmjq {input} {filter} {output}'''

PMJQ_BRANCH_MERGE_TEMPLATE = '''sudo -u {user} pmjq --inputs "{pattern}"\
{inputs} --cmd {cmd}'''

# Dot<->Petri net template from
# http://thegarywilson.com/blog/2011/drawing-petri-nets/
DOT_TEMPLATE = '''digraph G {{
subgraph place {{
graph [shape=circle,color=gray];
node [shape=circle];
{places}
}}

subgraph transitions {{
node [shape=rect];
{transitions}
}}

{place_trans_edges}

{trans_place_edges}
}}'''
