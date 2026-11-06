# Small-document A/B: xmltext before and after a change

`run.py` measures the stax-xml assets with two builds of the Go asset harness (`go-base`, this tree's xmltext; `go-new`, xmltext with `decoder.go` from another commit), Woodstox and quick-xml, in one session. Each implementation runs in two modes:

- `file`: read the file and parse it, as `../windows-amd64/assets-comparison` does.
- `mem`: read the file once and time only the parse and checksum. The difference between the modes is the cost of reading the file, which differs by language runtime and OS (Go's `os.ReadFile` costs about 1.1 µs more than plain open/read/close on Linux).

Each round starts one process per asset, mode and implementation, and the order of implementations rotates between rounds. Small assets are timed in batches (100 operations in `file` mode, 1000 in `mem` mode). The script checks the assets against `../windows-amd64/assets-comparison/assets.csv` and aborts unless all implementations give the same events and checksum for each asset.

Outputs, per platform directory: `samples.csv`, `summary.csv`, and `ratios.md`. The ratios divide throughputs, so a value above 1 means the first-named build is faster: `new÷base` is the change's effect, and the others compare each Go build with Woodstox and quick-xml in the same session.

Run from the repository root, with the toolchain built and the LF assets extracted by `../windows-amd64/assets-comparison/run.ps1`:

```powershell
./benchmark-results/small-ab/windows.ps1 -New <commit>
```

```sh
bash /mnt/g/programming/go-xml-v2/benchmark-results/small-ab/wsl.sh <commit>
```

The first change measured here is 02aa698c64 (`encoding/xml/xmltext: make the name cache after 128 names, not 32`).
