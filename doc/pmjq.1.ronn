pmjq(1) -- watch and process a job queue
========================================


## SYNOPSIS
`pmjq` [-C <cpu-limit> -o <archive-dir>] <job-spool>

## DESCRIPTION

The `pmjq` utility watches the directory <job-spool> and executes the scripts that exist in it. Once executed, the script is copied in the directory <archive-dir> if specified.

Users submit jobs by writing an executable file in the <job-spool> directory. Therefore, a minimal pmjq server is just a file server with write-access enabled. Users are responsible for choosing a naming scheme for their jobs that will avoid conflicts and collisions.

Running more than one instance of pmjq by server only makes sense if they are watching different pools. Multiple machines can mount the <job-spool> directory into their namespace and run `pmjq`, thus distributing the jobs among nodes of a cluster.

`pmjq` can not watch more than one pool at once. Funny business should be handled at the filesystem level, with e.g. something similar to Plan 9 bind(2).

The arguments are as follows:

* `-o`
Specify the archive directory, in which executed scripts are copied. A timestamp prefix is prepended to the name of the file. Jobs with the same name that finish executing at the same second will lead to data loss : one will overwrite the other. Users are responsible for choosing appropriate names.

* `-C`
Specify (as a percentage) a CPU limit. No new job is launched if the current CPU usage is above the limit.

## TODO

* Security considerations : allow jobs to be ran with sudo -u file-owner

* Allow a command to be run (e.g. shutting down the machine) if no jobs are available for a certain amount of time

* Allow a command to be run (e.g. booting new nodes) if one of the limit is reached for a certain amount of time

## BUGS

* `pmjq` has no mechanism to protect against name collisions in the <job-spool> directory, and only minimal (a timestamp) mechanism in the <archive-dir> directory

* The security is very very bad, write access to the file server should only be granted to trusted users.
