# lsfkit

`lsfkit` is a pure-Go LSF helper, replacing the `bsub.py` and
`bsub_out_to_stats` scripts from Farmpy.

```sh
lsfkit run --norun --memory-units MB 4 align 'aligner reads.fq'
lsfkit run --interactive --memory-units MB 4 interactive-shell bash
lsfkit ostats --all-columns --time-units h align.o
lsfkit ostats --summary *.o
lsfkit update --check
```

`run` submits LSF jobs, including arrays, dependencies, checkpoints, resource
tokens and memory requests. `ostats` reads LSF notification blocks; it safely
accepts an output file whose final notification lacks the traditional stderr
footer. Job names and filenames are always emitted in full.

`run --interactive` replaces `lsfkit` with `bsub -Is`, leaving bsub directly
attached to your terminal until the interactive command exits. It does not add
default `-o`/`-e` files, so the remote shell remains visible in your terminal.

`lsfkit update` downloads the latest GitHub release for the current platform,
verifies its SHA-256 checksum, and replaces the running binary. Use
`lsfkit update --check` to check without installing or `--force` when running a
development build.

Build locally with `./build.sh`; make all platform binaries with `./build.sh
--all`; and make release archives/checksums with `./build.sh --release --version
vX.Y.Z`. Pushing a matching version tag runs the same release process in GitHub
Actions.
