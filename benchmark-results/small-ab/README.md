# Small-document A/B: xmltext before and after a change

`run.py` measures the stax-xml assets with two builds of the Go asset harness (`go-base`, this tree's xmltext; `go-new`, xmltext with `decoder.go` from another commit), Woodstox and quick-xml, in one session. Each implementation runs in two modes:

- `file`: read the file and parse it, as `../windows-amd64/assets-comparison` does.
- `mem`: read the file once and time only the parse and checksum. The difference between the modes is the cost of reading the file, which differs by language runtime and OS (Go's `os.ReadFile` costs about 1.1 µs more than plain open/read/close on Linux).

Each round starts one process per asset, mode and implementation, and the order of implementations rotates between rounds. The default is nine rounds. `--cpus auto` (the default) pins the run, and every process it starts, to one logical CPU per performance core and sets `GOMAXPROCS` to that count; on a hybrid CPU this keeps a round from landing on efficiency cores. WSL does not expose core types and its virtual CPUs are scheduled by the host, so `wsl.sh` pins to fixed virtual CPUs (`0,2,4,6,8,10`) only. Small assets are timed in batches (100 operations in `file` mode, 1000 in `mem` mode). The script checks the assets against `../windows-amd64/assets-comparison/assets.csv` and aborts unless all implementations give the same events and checksum for each asset.

Outputs, per platform directory: `samples.csv`, `summary.csv`, and `ratios.md`. The ratios divide throughputs, so a value above 1 means the first-named build is faster. Each cell is the median of the per-round ratios followed by their range; a ratio whose range crosses 1 is not a measured difference. `new÷base` is the change's effect, and the others compare each Go build with Woodstox and quick-xml in the same session.

Run from the repository root, with the toolchain built and the LF assets extracted by `../windows-amd64/assets-comparison/run.ps1`:

```powershell
./benchmark-results/small-ab/windows.ps1 -New <commit>   # -Rounds 9 -Cpus auto -Out windows-amd64-pinned
```

```sh
bash /mnt/g/programming/go-xml-v2/benchmark-results/small-ab/wsl.sh <commit>   # [rounds=9] [cpus=0,2,4,6,8,10] [outdir=linux-amd64-pinned]
```

The first change measured here is 02aa698c64 (`encoding/xml/xmltext: make the name cache after 128 names, not 32`).

## 2026-11-06: Intel Core i5-13600K

Three rounds per platform; ratios compare median throughput within each platform session. Windows: Temurin Java 25.0.1+8, rustc 1.99.0 (b940084d7). WSL: Temurin Java 25.0.5+8, rustc 1.91.1 (ed61e7d7e). quick-xml 0.42.0; Woodstox 7.2.0.

### Windows amd64

| asset | mode | new÷base | base÷Woodstox | new÷Woodstox | base÷quick-xml | new÷quick-xml |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| mini-sample.xml | file | 0.96 | 1.25 | 1.20 | 0.88 | 0.85 |
| mini-sample.xml | mem | 1.00 | 1.96 | 1.96 | 0.86 | 0.87 |
| sample.xml | file | 1.01 | 1.21 | 1.22 | 0.85 | 0.85 |
| sample.xml | mem | 1.13 | 1.20 | 1.35 | 0.82 | 0.93 |
| complex.xml | file | 1.00 | 1.26 | 1.26 | 0.86 | 0.86 |
| complex.xml | mem | 1.11 | 1.04 | 1.16 | 0.77 | 0.86 |
| books.xml | file | 1.00 | 1.45 | 1.45 | 0.92 | 0.92 |
| books.xml | mem | 1.04 | 1.22 | 1.27 | 0.84 | 0.87 |
| ptest.xml | file | 0.99 | 1.30 | 1.29 | 0.85 | 0.84 |
| ptest.xml | mem | 1.19 | 1.19 | 1.41 | 0.58 | 0.69 |
| crlf.xml | file | 0.90 | 1.28 | 1.14 | 0.93 | 0.83 |
| crlf.xml | mem | 1.00 | 2.23 | 2.23 | 0.77 | 0.77 |
| midsize.xml | file | 1.00 | 1.16 | 1.16 | 1.04 | 1.04 |
| midsize.xml | mem | 1.00 | 1.12 | 1.12 | 1.02 | 1.02 |
| large.xml | file | 1.00 | 1.17 | 1.17 | 1.04 | 1.04 |
| large.xml | mem | 1.00 | 1.12 | 1.12 | 1.02 | 1.02 |

### WSL linux amd64

| asset | mode | new÷base | base÷Woodstox | new÷Woodstox | base÷quick-xml | new÷quick-xml |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| mini-sample.xml | file | 1.13 | 1.98 | 2.24 | 0.52 | 0.58 |
| mini-sample.xml | mem | 1.01 | 2.04 | 2.06 | 0.72 | 0.73 |
| sample.xml | file | 1.02 | 2.09 | 2.14 | 0.66 | 0.68 |
| sample.xml | mem | 1.15 | 1.42 | 1.63 | 0.76 | 0.87 |
| complex.xml | file | 0.98 | 2.13 | 2.09 | 0.66 | 0.65 |
| complex.xml | mem | 1.14 | 1.20 | 1.36 | 0.70 | 0.79 |
| books.xml | file | 1.00 | 2.12 | 2.13 | 0.76 | 0.76 |
| books.xml | mem | 1.05 | 1.43 | 1.50 | 0.85 | 0.89 |
| ptest.xml | file | 1.12 | 2.11 | 2.38 | 0.46 | 0.52 |
| ptest.xml | mem | 1.23 | 1.20 | 1.48 | 0.53 | 0.65 |
| crlf.xml | file | 0.98 | 2.66 | 2.61 | 0.49 | 0.48 |
| crlf.xml | mem | 1.02 | 2.65 | 2.70 | 0.59 | 0.60 |
| midsize.xml | file | 1.01 | 1.17 | 1.17 | 1.02 | 1.03 |
| midsize.xml | mem | 0.99 | 1.14 | 1.13 | 1.06 | 1.05 |
| large.xml | file | 1.01 | 1.13 | 1.14 | 1.08 | 1.09 |
| large.xml | mem | 1.00 | 1.10 | 1.10 | 1.05 | 1.05 |
