#!/usr/bin/env bash
# Run the GOEXPERIMENT=xmlv2 reader benchmark on a fresh Linux machine, such
# as an Ubuntu live USB session. Needs only bash, python3, tar, coreutils,
# taskset and network access; it downloads everything else into $WORK.
# See README.md next to this file.
set -euo pipefail

GO_COMMIT=c759c45a388b282e324d7cd9404e5a078e490d34   # xml-v2-poc
GO_COMMIT_DATE="Sun Nov 8 08:06:42 2026 +0900"
BOOT_VERSION=go1.26.0                                 # minimum bootstrap for this tree
FIXTURE_SHA256=1285dbb0d198c4c5e579faa626f7ef02b0b279b1182d37d4039f3acb5929d7d4

ROUNDS=${ROUNDS:-20}       # process pairs; each pair runs both builds, order alternating
WARMUPS=${WARMUPS:-3}      # untimed runs per case per process
RUNS=${RUNS:-10}           # timed runs per case per process
NO_TURBO=${NO_TURBO:-0}    # 1: turn off Intel turbo while measuring
WORK=${WORK:-$HOME/xmlbench}
KIT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
HARNESS=$KIT/../darwin-arm64   # the same Go harness as the darwin/arm64 run

log() { printf '\n== %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

fetch() { # url dest
	if command -v curl >/dev/null; then curl -fL --retry 3 -o "$2" "$1"
	elif command -v wget >/dev/null; then wget -q --show-progress --progress=bar:force:noscroll -O "$2" "$1"
	else python3 -c 'import sys,urllib.request; urllib.request.urlretrieve(sys.argv[1], sys.argv[2])' "$1" "$2"
	fi
}

case $(uname -m) in
	x86_64) ARCH=amd64; BOOT_SHA256=aac1b08a0fb0c4e0a7c1555beb7b59180b05dfc5a3d62e40e9de90cd42f88235 ;;
	aarch64) ARCH=arm64; BOOT_SHA256=bd03b743eb6eb4193ea3c3fd3956546bf0e3ca5b7076c8226334afe6b75704cd ;;
	*) die "unsupported machine $(uname -m)" ;;
esac
[ "$(uname -s)" = Linux ] || die "Linux only"
for c in python3 tar sha256sum taskset; do command -v $c >/dev/null || die "missing $c"; done
[ -f "$HARNESS/main.go" ] || die "harness not found at $HARNESS"

mkdir -p "$WORK"
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
OUT=$WORK/results-$STAMP
mkdir -p "$OUT/raw"
exec > >(tee "$OUT/run.log") 2>&1
echo "work dir: $WORK"
echo "results:  $OUT"
avail=$(df -Pm "$WORK" | awk 'NR==2 {print $4}')
[ "$avail" -ge 2500 ] || echo "warning: only ${avail} MiB free in $WORK; about 2.5 GiB is needed"

# 1. Bootstrap Go.
BOOT=$WORK/$BOOT_VERSION
if [ ! -x "$BOOT/bin/go" ]; then
	log "download $BOOT_VERSION (bootstrap)"
	fetch "https://go.dev/dl/$BOOT_VERSION.linux-$ARCH.tar.gz" "$WORK/boot.tgz"
	echo "$BOOT_SHA256  $WORK/boot.tgz" | sha256sum -c -
	rm -rf "$BOOT.tmp" && mkdir -p "$BOOT.tmp"
	tar -xzf "$WORK/boot.tgz" -C "$BOOT.tmp"
	mv "$BOOT.tmp/go" "$BOOT" && rm -rf "$BOOT.tmp" "$WORK/boot.tgz"
fi

# 2. The xml-v2-poc toolchain.
GOROOT_X=$WORK/go-$GO_COMMIT
GO=$GOROOT_X/bin/go
if [ ! -x "$GO" ]; then
	log "download xml-v2-poc at ${GO_COMMIT:0:10}"
	fetch "https://codeload.github.com/Clickin/go-xml-v2/tar.gz/$GO_COMMIT" "$WORK/src.tgz"
	rm -rf "$GOROOT_X" "$WORK/go-xml-v2-$GO_COMMIT"
	tar -xzf "$WORK/src.tgz" -C "$WORK"
	mv "$WORK/go-xml-v2-$GO_COMMIT" "$GOROOT_X" && rm "$WORK/src.tgz"
	# No .git in a tarball: give cmd/dist the version it would read from git.
	# (VERSION, not VERSION.cache: make.bash deletes VERSION.cache.)
	echo "go1.28-devel_${GO_COMMIT:0:10} $GO_COMMIT_DATE" > "$GOROOT_X/VERSION"
	log "build the toolchain (a few minutes)"
	(cd "$GOROOT_X/src" && GOROOT_BOOTSTRAP=$BOOT ./make.bash)
fi
export GOROOT=$GOROOT_X GOPATH=$WORK/gopath GOCACHE=$WORK/gocache GOTOOLCHAIN=local
unset GOEXPERIMENT
"$GO" version

# 3. Fixture.
FIXTURE=$WORK/runtime-comparison-16mib.xml
if [ ! -f "$FIXTURE" ] || ! echo "$FIXTURE_SHA256  $FIXTURE" | sha256sum -c --quiet - 2>/dev/null; then
	log "generate the 16 MiB fixture"
	python3 "$KIT/gen16mib.py" "$FIXTURE"
fi
echo "$FIXTURE_SHA256  $FIXTURE" | sha256sum -c -

# 4. Harness binaries.
log "build the harness for both builds"
BUILD=$WORK/harness
rm -rf "$BUILD" && mkdir -p "$BUILD"
cp "$HARNESS"/go.mod "$HARNESS"/*.go "$BUILD"/
# The baseline is built with GOEXPERIMENT unset (GOEXPERIMENT=none would also
# turn off experiments that are on by default).
(cd "$BUILD" && env -u GOEXPERIMENT "$GO" build -trimpath -o "$WORK/bench-none" . \
	&& GOEXPERIMENT=xmlv2 "$GO" build -trimpath -o "$WORK/bench-xmlv2" .)
"$GO" version -m "$WORK/bench-xmlv2" | grep -E 'GOEXPERIMENT|^\s+go\s' || true

# 5. Machine settings. Restored on exit.
CPUS=$(cat /sys/devices/cpu_core/cpus 2>/dev/null || true)   # performance cores on hybrid Intel
CPU_NOTE="performance cores (/sys/devices/cpu_core/cpus)"
if [ -z "$CPUS" ]; then CPUS=0-$(( $(nproc --all) - 1 )); CPU_NOTE="all CPUs (not a hybrid CPU)"; fi
SUDO=""; if [ "$(id -u)" != 0 ]; then sudo -n true 2>/dev/null && SUDO="sudo -n"; fi
GOVS=(/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor)
OLD_GOV=$(cat "${GOVS[0]}" 2>/dev/null || echo unknown)
TURBO=/sys/devices/system/cpu/intel_pstate/no_turbo
OLD_TURBO=$(cat $TURBO 2>/dev/null || echo unknown)
restore() {
	[ "$OLD_GOV" = unknown ] || for g in "${GOVS[@]}"; do echo "$OLD_GOV" | $SUDO tee "$g" >/dev/null 2>&1 || true; done
	[ "$OLD_TURBO" = unknown ] || echo "$OLD_TURBO" | $SUDO tee $TURBO >/dev/null 2>&1 || true
}
trap restore EXIT
if [ "$OLD_GOV" != unknown ]; then
	for g in "${GOVS[@]}"; do echo performance | $SUDO tee "$g" >/dev/null 2>&1 || true; done
fi
if [ "$NO_TURBO" = 1 ] && [ "$OLD_TURBO" != unknown ]; then echo 1 | $SUDO tee $TURBO >/dev/null 2>&1 || true; fi

{
	echo "date_utc: $STAMP"
	echo "kernel: $(uname -r)"
	grep -m1 'model name' /proc/cpuinfo || true
	echo "pinned_cpus: $CPUS ($CPU_NOTE)"
	echo "governor: $(cat "${GOVS[0]}" 2>/dev/null || echo unknown) (was $OLD_GOV)"
	echo "intel_pstate no_turbo: $(cat $TURBO 2>/dev/null || echo unknown) (was $OLD_TURBO)"
	grep -E 'MemTotal' /proc/meminfo || true
	echo "virtualization: $(systemd-detect-virt 2>/dev/null || echo unknown)"
	echo "go: $("$GO" version)"
	echo "xml-v2-poc: $GO_COMMIT"
	echo "fixture: $FIXTURE_SHA256"
	echo "rounds: $ROUNDS, warmups: $WARMUPS, runs: $RUNS"
	[ -f /etc/os-release ] && grep -E '^PRETTY_NAME' /etc/os-release || true
} | tee "$OUT/environment.txt"
lscpu > "$OUT/lscpu.txt" 2>/dev/null || true

# 6. Measure. Each round runs both builds in separate processes, alternating
# which goes first, so drift over the session hits both builds alike.
log "measure: $ROUNDS rounds"
for r in $(seq 1 "$ROUNDS"); do
	if [ $((r % 2)) = 1 ]; then order="none xmlv2"; else order="xmlv2 none"; fi
	for b in $order; do
		taskset -c "$CPUS" "$WORK/bench-$b" -file "$FIXTURE" -warmups "$WARMUPS" -runs "$RUNS" \
			> "$OUT/raw/round$(printf %02d "$r")-$b.json"
	done
	printf 'round %d/%d done\n' "$r" "$ROUNDS"
done

# 7. Summarize.
log "summarize"
CPU_NAME=$(grep -m1 'model name' /proc/cpuinfo | sed 's/.*: //' || echo unknown)
python3 "$KIT/summarize.py" "$OUT" "$ARCH" "$CPU_NAME"
if [ ! -x "$WORK/bin/benchstat" ]; then
	GOBIN=$WORK/bin GOROOT=$BOOT GOTOOLCHAIN=local "$BOOT/bin/go" install golang.org/x/perf/cmd/benchstat@latest \
		|| echo "warning: could not install benchstat; summary.md still has the numbers"
fi
if [ -x "$WORK/bin/benchstat" ]; then
	(cd "$OUT" && "$WORK/bin/benchstat" none=none.txt xmlv2=xmlv2.txt | tee benchstat.txt)
fi

tar -czf "$WORK/xmlbench-results-$STAMP.tar.gz" -C "$WORK" "results-$STAMP"
log "done"
echo "Results: $OUT"
echo "Archive: $WORK/xmlbench-results-$STAMP.tar.gz"
echo "A live session keeps files in RAM only. Copy the archive to a USB drive or another disk before shutting down."
