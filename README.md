# lsfkit

`lsfkit` is a pure-Go command-line tool for submitting LSF jobs and collecting
statistics from their output files. It provides `run` for job submission and
`ostats` for reading LSF job notifications. This repository was developed with
substantial coding assistance from [OpenAI Codex](https://openai.com/codex) and
[Claude Code](https://claude.com/claude-code), which helped with
implementation, refactoring, tests, documentation, and benchmarking under
human direction and review.

## Install

Download a prebuilt Linux binary from the [latest GitHub
release](https://github.com/martinghunt/lsfkit/releases/latest). Releases are
available for Linux amd64 and arm64, with a SHA-256 checksum file for verifying
the downloaded archive.

After extracting the archive, place `lsfkit` somewhere on your `PATH`, then
check the installation:

```bash
lsfkit --version
```

`run` requires the LSF command-line client, including `bsub`. It uses `lsadmin`
to discover the LSF memory unit unless you provide `--memory-units` or set
`LSFKIT_LSF_MEMORY_UNITS` to `KB` or `MB`. The command-line option takes
precedence over the environment variable. `ostats` only reads local LSF output
files and does not require an LSF installation.

To update an installed release binary:

```bash
lsfkit update
lsfkit update --check
```

`update` verifies the downloaded archive against the published SHA-256
checksum before replacing the binary. It is available for the Linux release
platforms; a local `dev` build requires `--force` to update.

To build locally instead:

```bash
./build.sh
```

That builds for the current OS and architecture in `./build/`. This is useful
for local development and command-line testing on macOS. To cross-compile a
specific target, use `--os` and `--arch`; `./build.sh --all` builds the complete
test matrix. `./build.sh --release --version vX.Y.Z` creates the Linux release
archives and checksums.

## Usage

`lsfkit` has four commands:

- `lsfkit run`: submit an LSF job
- `lsfkit array`: submit an LSF job array from a file of commands
- `lsfkit ostats`: report statistics from LSF output files
- `lsfkit update`: update an installed Linux release binary

Use `lsfkit --help` for top-level help and `lsfkit COMMAND --help` for the
options of a command.

### Submit jobs with `run`

The basic form is:

```bash
lsfkit run [options] <memory-gb> <job-name> <command> [command-arguments...]
```

`memory-gb` is a non-negative decimal value in GB. Options must appear before
the positional arguments. Unless overridden with `--out` and `--err`, output is
written to `<job-name>.o` and `<job-name>.e`. Full job names and filenames are
passed to LSF unchanged.

Examples:

```bash
# Submit a one-thread job using 4 GB and the LSF memory unit reported by lsadmin.
lsfkit run 4 align aligner reads.fq

# Make the memory-unit choice explicit and choose a queue and log files.
lsfkit run --memory-units MB --queue long --out logs/align.o --err logs/align.e \
  8 align aligner reads.fq

# Inspect the bsub command without submitting anything.
lsfkit run --norun --memory-units MB 4 align aligner reads.fq

# Request four threads, temporary space, and a named resource token.
lsfkit run --memory-units MB --threads 4 --tmp-space 20 \
  --tokens-name license --tokens-number 1 \
  16 assemble assembler reads.fq

# Submit an array. INDEX in each command argument becomes $LSB_JOBINDEX.
lsfkit run --memory-units MB --start 1 --end 100 --array-limit 20 \
  2 map mapper reads_INDEX.fq

# Wait for other jobs. Names may be supplied repeatedly.
lsfkit run --memory-units MB --done prepare --done index --ended cleanup \
  4 analyse analysis input.dat
```

For an interactive shell, `run --interactive` invokes `bsub -Is` and replaces
the `lsfkit` process, so your terminal remains directly connected to the
interactive job until it exits. It does not create default output or error
files:

```bash
lsfkit run --interactive --memory-units MB 0.5 shell bash
```

### Submit command files with `array`

Use `array` when each line of a shared file is a shell command to run in a
separate LSF array element:

```bash
lsfkit array [options] <memory-gb> <job-name> <commands-file>
```

The first line is run by array element 1, the second by element 2, and so on.
The commands file must be accessible on the execution hosts for the lifetime
of the array. Empty or whitespace-only lines are rejected.

```bash
# commands.txt contains one command per line, such as: foo > bar
lsfkit array --memory-units MB --array-limit 20 2 map commands.txt

# -o and -e are prefixes for array logs, so the two directories can differ.
lsfkit array --memory-units MB -o logs/out/map -e logs/err/map \
  2 map commands.txt
# Creates logs/out/map.1.o and logs/err/map.1.e for the first array element.
```

Without `-o` or `-e`, logs use the job name as their prefix, for example
`map.1.o` and `map.1.e`. Commands are interpreted by `sh -c`, so ordinary
shell syntax such as quotes, pipes, and redirection works. Commands must each
fit on one physical line. Each array element writes its selected line to its
standard-output log before running it.

### Read output statistics with `ostats`

`ostats` reads the LSF notification blocks in one or more output files and
writes tab-separated output. Time columns are rounded to two decimal places.
By default it reports the one-based `number_in_file`, exit code, CPU time,
wall-clock time, peak memory, requested memory, and source filename.

```bash
# Report the standard columns in hours.
lsfkit ostats align.o assemble.o

# Use minutes and include every available column.
lsfkit ostats --time-units m --all-columns *.o

# Show only failed jobs.
lsfkit ostats --fails *.o

# Retain a placeholder row for output files with no LSF notification.
lsfkit ostats --include-no-data *.o

# Summarize exit codes instead of printing one row per job. --fails is
# ignored here: the summary always covers every exit code.
lsfkit ostats --summary *.o

# Write tabular output to a file.
lsfkit ostats --outfile job-stats.tsv *.o
```

`--time-units` accepts `s`, `m`, or `h` and defaults to `h`. `--all-columns`
also includes process and thread counts, timestamps, execution host, user,
working directory, and job name. `ostats` safely handles output files whose
last LSF notification is incomplete, including files to which a later rerun
has appended another notification. By default files without usable LSF job data
are omitted from tabular output; use `--include-no-data` to emit a row whose
statistic columns are `*` for each such file. `number_in_file` identifies the
one-based LSF notification position within its source file.
