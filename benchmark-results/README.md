# XML parser benchmarks: Windows and WSL Linux amd64

The `windows-amd64` directory contains the original Go benchmark suite and the comparisons with Woodstox and quick-xml. The most directly comparable cross-platform experiment uses eight identical files from stax-xml and is recorded in `windows-amd64/assets-comparison` and `linux-amd64`. Both runs use Go revision `bd5149100704087efd1edf06bfb7d6956aa636ac`, Woodstox 7.2.0 and quick-xml 0.42.0 on the same Intel i5-13600K. All 24 implementation/asset event counts and checksums match across operating systems. Asset sizes and SHA-256 hashes are in `windows-amd64/assets-comparison/assets.csv`.

| Asset | Size | Windows Go | Windows Woodstox | Windows quick-xml | WSL Go | WSL Woodstox | WSL quick-xml |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| mini-sample.xml | 542 B | 16.6 | 12.5 | 17.9 | 72.2 | 41.7 | 139.7 |
| sample.xml | 1,555 B | 36.8 | 32.7 | 42.9 | 107.7 | 90.1 | 201.3 |
| complex.xml | 1,607 B | 42.2 | 34.7 | 45.4 | 132.3 | 86.8 | 206.4 |
| books.xml | 4,551 B | 83.7 | 72.2 | 99.5 | 197.8 | 129.4 | 289.2 |
| ptest.xml | 1,362 B | 37.0 | 28.6 | 44.0 | 174.5 | 85.4 | 336.4 |
| crlf.xml | 335 B | 10.4 | 8.5 | 11.6 | 67.0 | 25.5 | 141.3 |
| midsize.xml | 14,017,551 B | 196.9 | 223.2 | 198.2 | 197.2 | 253.8 | 222.9 |
| large.xml | 105,131,540 B | 200.4 | 249.5 | 201.3 | 197.6 | 250.1 | 209.1 |

Values are median MiB/s for the file-read-plus-parse-and-checksum operation. Small files are measured in batches of 100 operations to avoid timer resolution issues. See the [Windows asset methodology](windows-amd64/assets-comparison/README.md), [Windows samples](windows-amd64/assets-comparison/samples.csv), and [Linux methodology and samples](linux-amd64/README.md).

These are end-to-end, warm-cache workload measurements, not isolated parser speed. WSL reads inputs from its Linux ext4 filesystem, while Windows uses NTFS; filesystem and system-call costs can dominate small files. The WSL runs use Java 25.0.5 and Rust 1.91.1, while Windows uses Java 25.0.1 and Rust 1.99.0, so cross-platform differences cannot be attributed solely to the OS or WSL. The parsers differ in validation and namespace behavior, and matching checksums do not prove full XML semantic equivalence. Repeated measurements within one process are not independent. No allocation, cold-cache, or statistical significance claim is made.
