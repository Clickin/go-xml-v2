# Bare-metal Linux run kit

This kit runs the `encoding/xml` reader benchmark from [`../darwin-arm64`](../darwin-arm64/README.md) on a fresh Linux machine with no setup, such as an Ubuntu live USB session. It compares the current `encoding/xml` with `GOEXPERIMENT=xmlv2` on the same 16 MiB document. Unlike the WSL runs in `../linux-amd64`, nothing is virtualized.

Korean instructions: [`README.ko.md`](README.ko.md).

## What it needs

- Linux on x86-64 (arm64 also works), with network access
- `bash`, `python3`, `tar`, `sha256sum`, `taskset` (all present on an Ubuntu live session)
- about 2.5 GiB free in `$WORK` (default `~/xmlbench`); on a live session this is RAM

It does not install system packages and does not need git.

## Run

```sh
cd ~
wget -O kit.tgz https://codeload.github.com/Clickin/go-xml-v2/tar.gz/refs/heads/benchmarks/windows-linux-amd64
tar -xzf kit.tgz
bash go-xml-v2-benchmarks-windows-linux-amd64/benchmark-results/linux-live/run.sh
```

`curl -L -o kit.tgz <same URL>` works too.

## What `run.sh` does

1. Downloads Go 1.26.0 (the minimum bootstrap for this tree) from go.dev and checks its SHA-256.
2. Downloads the `xml-v2-poc` source at `c759c45a38` and builds the toolchain with `make.bash`. A tarball has no `.git`, so the script writes `VERSION` as `go1.28-devel_c759c45a38 <commit date>`.
3. Generates the fixture with `gen16mib.py`, a port of `generateXmlFile` in Clickin/stax-xml `packages/benchmark/runtime-matrix.mjs` (07cca8a0). It checks the SHA-256 (`1285dbb0…d7d4`, the same file as the darwin/arm64 run).
4. Builds the harness in `../darwin-arm64` twice: with `GOEXPERIMENT` unset (current) and with `GOEXPERIMENT=xmlv2`.
5. Sets the CPU governor to `performance` (through `sudo -n`, skipped if that fails), and pins the processes to the performance cores listed in `/sys/devices/cpu_core/cpus` on hybrid Intel CPUs (all CPUs otherwise). Settings are restored on exit.
6. Runs 20 rounds. Each round runs both builds in separate processes, alternating which runs first. Each process does 3 warmups and 10 timed runs per case. Each timed run reads the file and parses it.
7. Checks that every reader produced the expected events and checksum (967,965 / 36,104,832), then writes `summary.md` and benchstat input (`none.txt`, `xmlv2.txt`, one sample per process). If benchstat can be installed, it also writes `benchstat.txt`.

Settings, as environment variables:

| Variable | Default | Meaning |
|---|---|---|
| `ROUNDS` | 20 | process pairs |
| `WARMUPS` | 3 | untimed runs per case per process |
| `RUNS` | 10 | timed runs per case per process |
| `NO_TURBO` | 0 | `1` turns off Intel turbo while measuring |
| `WORK` | `~/xmlbench` | download, build and results directory |

## Output

`$WORK/results-<UTC time>/` and the same directory as `$WORK/xmlbench-results-<UTC time>.tar.gz`:

- `summary.md`: medians, MiB/s, allocations, and the current ÷ xmlv2 ratio per round
- `benchstat.txt`, `none.txt`, `xmlv2.txt`
- `environment.txt`, `lscpu.txt`, `run.log`
- `raw/`: the JSON output of every process

A live session keeps everything in RAM. Copy the archive to a USB drive or another disk before shutting down.
