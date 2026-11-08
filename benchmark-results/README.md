# XML parser benchmarks: Windows and WSL Linux amd64

The darwin/arm64 run behind the `encoding/xml` numbers in the `GOEXPERIMENT=xmlv2` issue (one 16 MiB document, Apple M4) is in [`darwin-arm64`](darwin-arm64/README.md).

The same comparison on bare-metal Linux amd64 (Intel i5-13600K, Ubuntu live USB session, c759c45a38) is in [`linux-live`](linux-live/README.md#results): `Decoder.Token` 2.25× and `Unmarshal` 2.41× faster under `GOEXPERIMENT=xmlv2`, over 20 interleaved process pairs. The same directory has the kit to repeat it on any Linux machine with nothing installed.

The `windows-amd64` directory contains the original Go benchmark suite and the comparisons with Woodstox and quick-xml. The most directly comparable cross-platform experiment uses eight identical files from stax-xml and is recorded in `windows-amd64/assets-comparison` and `linux-amd64`. Both runs use Go revision `bd5149100704087efd1edf06bfb7d6956aa636ac`, Woodstox 7.2.0 and quick-xml 0.42.0 on the same Intel i5-13600K. All 24 implementation/asset event counts and checksums match across operating systems. Asset sizes and SHA-256 hashes are in `windows-amd64/assets-comparison/assets.csv`.

| Asset | Size (bytes) | Windows Go MiB/s | Woodstox MiB/s | quick-xml MiB/s | Go÷Woodstox | Go÷quick-xml | WSL Go MiB/s | Woodstox MiB/s | quick-xml MiB/s | Go÷Woodstox | Go÷quick-xml |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| mini-sample.xml | 525 | 14.2 | 12.9 | 17.4 | 1.10× | 0.82× | 82.7 | 31.9 | 150.8 | 2.59× | 0.55× |
| sample.xml | 1,508 | 38.0 | 27.2 | 43.1 | 1.40× | 0.88× | 128.9 | 71.9 | 213.9 | 1.79× | 0.60× |
| complex.xml | 1,560 | 37.0 | 33.0 | 47.3 | 1.12× | 0.78× | 144.7 | 52.0 | 222.6 | 2.78× | 0.65× |
| books.xml | 4,431 | 102.5 | 70.3 | 109.7 | 1.46× | 0.93× | 238.1 | 96.3 | 312.6 | 2.47× | 0.76× |
| ptest.xml | 1,330 | 36.1 | 27.2 | 43.1 | 1.33× | 0.84× | 159.1 | 72.2 | 332.1 | 2.20× | 0.48× |
| crlf.xml | 327 | 9.9 | 8.1 | 11.9 | 1.22× | 0.83× | 68.0 | 27.0 | 140.1 | 2.52× | 0.49× |
| midsize.xml | 13,584,398 | 271.5 | 217.7 | 246.4 | 1.25× | 1.10× | 267.6 | 229.1 | 251.0 | 1.17× | 1.07× |
| large.xml | 101,882,907 | 261.1 | 233.9 | 255.2 | 1.12× | 1.02× | 269.1 | 226.3 | 234.6 | 1.19× | 1.15× |

Throughput values are median MiB/s; ratios divide the Go median throughput by the corresponding implementation median in the same OS and session (larger means Go is faster). Values are measured for the file-read-plus-parse-and-checksum operation. Small files are measured in batches of 100 operations to avoid timer resolution issues. See the [Windows asset methodology](windows-amd64/assets-comparison/README.md), [Windows samples](windows-amd64/assets-comparison/samples.csv), and [Linux methodology and samples](linux-amd64/README.md).

These are end-to-end, warm-cache workload measurements, not isolated parser speed. WSL reads inputs from its Linux ext4 filesystem, while Windows uses NTFS; filesystem and system-call costs can dominate small files. The WSL runs use Java 25.0.5 and Rust 1.91.1, while Windows uses Java 25.0.1 and Rust 1.99.0, so cross-platform differences cannot be attributed solely to the OS or WSL. The parsers differ in validation and namespace behavior, and matching checksums do not prove full XML semantic equivalence. Repeated measurements within one process are not independent. No allocation, cold-cache, or statistical significance claim is made.
