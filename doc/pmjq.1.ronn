pmjq(1) -- watch and process a job queue
========================================


## SYNOPSIS
`pmjq` [-C <cpu-limit> -o <archive-dir>] <job-spool>

## DESCRIPTION

The `pmjq` utility watches the directory <job-spool> and executes the scripts that exist in it. Once executed, the script is copied in the directory <archive-dir> if specified.

The arguments are as follows:

* `-o`
Specify the archive directory, in which executed scripts are copied.

* `-C`
Specify (as a percentage) a CPU limit. No new job is launched if the current CPU usage is above the limit.

