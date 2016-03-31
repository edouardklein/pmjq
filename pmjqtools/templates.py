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
