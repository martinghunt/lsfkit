# lsfkit

`lsfkit` is a pure-Go LSF helper, replacing the `bsub.py` and
`bsub_out_to_stats` scripts from Farmpy.

```sh
lsfkit run --norun --memory-units MB 4 align 'aligner reads.fq'
lsfkit ostats --all-columns --time-units h align.o
lsfkit ostats --summary *.o
```

`run` submits LSF jobs, including arrays, dependencies, checkpoints, resource
tokens and memory requests. `ostats` reads LSF notification blocks; it safely
accepts an output file whose final notification lacks the traditional stderr
footer. Job names and filenames are always emitted in full.

Build locally with `./build.sh`; make all platform binaries with `./build.sh
--all`; and make release archives/checksums with `./build.sh --release --version
vX.Y.Z`. Pushing a matching version tag runs the same release process in GitHub
Actions.
