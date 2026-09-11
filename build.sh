#!/usr/bin/env bash

set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bin_name="lsfkit"
release=0
all=0
version=""
output_dir=""
requested_os=""
requested_arch=""
artifacts=()

usage() {
	cat <<'EOF'
Usage:
  ./build.sh
  ./build.sh --os linux --arch arm64
  ./build.sh --all
  ./build.sh --release --version v1.2.3

Default behaviour builds the host binary in ./build.

Options:
  --all               Build darwin, linux and windows for amd64 and arm64.
  --release           Build, archive and checksum the full platform matrix.
  --version VERSION   Version in artifacts; required with --release.
  --os GOOS           Build one operating system (defaults to host OS).
  --arch GOARCH       Build one architecture (defaults to host architecture).
  --output-dir DIR    Override build/ or dist/.
  -h, --help          Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--release) release=1; shift ;;
		--all) all=1; shift ;;
		--version) version="${2:?missing version}"; shift 2 ;;
		--os) requested_os="${2:?missing OS}"; shift 2 ;;
		--arch) requested_arch="${2:?missing architecture}"; shift 2 ;;
		--output-dir) output_dir="${2:?missing output directory}"; shift 2 ;;
		-h|--help) usage; exit 0 ;;
		*) echo "unknown argument: $1" >&2; usage >&2; exit 1 ;;
	esac
done

if [[ $release -eq 1 && -z "$version" ]]; then
	echo "--release requires --version" >&2
	exit 1
fi
if [[ $release -eq 1 && ( -n "$requested_os" || -n "$requested_arch" ) ]]; then
	echo "--release cannot be combined with --os/--arch" >&2
	exit 1
fi
if [[ $all -eq 1 && ( -n "$requested_os" || -n "$requested_arch" ) ]]; then
	echo "--all cannot be combined with --os/--arch" >&2
	exit 1
fi

if [[ -z "$output_dir" ]]; then
	if [[ $release -eq 1 || $all -eq 1 ]]; then
		output_dir="$root_dir/dist"
	else
		output_dir="$root_dir/build"
	fi
fi

mkdir -p "$output_dir" "$root_dir/.cache/gocache"
export GOCACHE="${GOCACHE:-$root_dir/.cache/gocache}"

package_release_artifact() {
	local goos="$1"
	local outfile="$2"
	local goarch="$3"
	local archive_base="$bin_name-$version-$goos-$goarch"

	if [[ "$goos" == "windows" ]]; then
		archive_base+=".exe"
		(cd "$output_dir" && zip -q -m "$archive_base.zip" "$(basename "$outfile")")
		artifacts+=("$archive_base.zip")
	else
		tar -C "$output_dir" -czf "$output_dir/$archive_base.tar.gz" "$(basename "$outfile")"
		rm -f "$outfile"
		artifacts+=("$archive_base.tar.gz")
	fi
}

build_one() {
	local goos="$1"
	local goarch="$2"
	local extension=""
	local suffix
	local outfile

	if [[ "$goos" == "windows" ]]; then
		extension=".exe"
	fi
	suffix="${version:+-$version}-$goos-$goarch$extension"
	outfile="$output_dir/$bin_name$suffix"
	if [[ $release -eq 0 && $all -eq 0 && -z "$requested_os" && -z "$requested_arch" ]]; then
		outfile="$output_dir/$bin_name$extension"
	fi

	echo "building $goos/$goarch -> ${outfile#$root_dir/}"
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
		go build -trimpath \
			-ldflags "-X github.com/martinghunt/lsfkit/internal/buildinfo.Version=${version:-dev}" \
			-o "$outfile" ./cmd/lsfkit

	if [[ $release -eq 1 ]]; then
		package_release_artifact "$goos" "$outfile" "$goarch"
	fi
}

if [[ $release -eq 1 || $all -eq 1 ]]; then
	for goos in darwin linux windows; do
		for goarch in amd64 arm64; do
			build_one "$goos" "$goarch"
		done
	done
else
	build_one "${requested_os:-$(go env GOOS)}" "${requested_arch:-$(go env GOARCH)}"
fi

if [[ $release -eq 1 ]]; then
	checksum="$output_dir/$bin_name-$version-checksums.txt"
	: > "$checksum"
	for artifact in "${artifacts[@]}"; do
		if command -v sha256sum >/dev/null; then
			(cd "$output_dir" && sha256sum "$artifact") >> "$checksum"
		else
			(cd "$output_dir" && shasum -a 256 "$artifact") >> "$checksum"
		fi
	done
	printf 'wrote %s\n' "${checksum#$root_dir/}"
fi
