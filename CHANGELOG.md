# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Add `lsfkit batch` to submit job lists with memory limits, per-job `run` options, and dependencies on earlier jobs (`done` by default or `ended:name`).
- Support batch input from files or stdin, comments and blank lines, full validation before submission, job-ID dependencies, and `--norun` previews.

## [0.5.0] - 2026-10-14

### Added
- Add `lsfkit ostats --outfmt` with `json`, `table`, `tsv`, `ttable`, and `ttsv` formats for job statistics and exit-code summaries.

### Changed
- Make `table` the default output format for `lsfkit ostats`; use `--outfmt tsv` for tab-separated output.

## [0.4.0] - 2026-09-16

### Added
- Add `lsfkit array` for submitting one shell command per line as an LSF job array.

### Removed
- Remove BLCR checkpointing support (`run --checkpoint`, `--checkpoint-dir`, `--checkpoint-period`).

## [0.3.0] - 2026-09-11

### Added
- Add the one-based `number_in_file` column to `lsfkit ostats` output.

## [0.2.0] - 2026-09-11

### Added
- Add `lsfkit ostats --include-no-data` to emit a placeholder row for output files without usable LSF job data.

### Changed
- Make `lsfkit --version` print `lsfkit vX.Y.Z`.

## [0.1.0] - 2026-09-11

Release `v0.1.0`, before changelog tracking started in this file.

[Unreleased]: https://github.com/martinghunt/lsfkit/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/martinghunt/lsfkit/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/martinghunt/lsfkit/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/martinghunt/lsfkit/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/martinghunt/lsfkit/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/martinghunt/lsfkit/releases/tag/v0.1.0
