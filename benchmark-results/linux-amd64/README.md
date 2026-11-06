# WSL Linux amd64 asset benchmark

Ran on Ubuntu WSL2, Linux 6.6.114.1-microsoft-standard-WSL2, x86-64, Intel Core i5-13600K. Exact toolchain versions and settings are in `environment.json`; Go XML tests are in `tests.txt`. The Go, Rust and Java programs are the same sources used for the Windows asset benchmark. Java heap is fixed at 512 MiB. The Windows asset methodology, including the namespace/checksum adaptations, warmups and sample counts, also applies here: see `../windows-amd64/assets-comparison/README.md`.

`setup.sh` copies the eight asset files byte-for-byte from the Windows checkout into `/tmp/go-xml-v2-benchmark/assets`, builds Go in a separate clone in `/tmp`, and installs temporary Go bootstrap and Temurin JDK there. `run.sh` builds the benchmark executables and copies the Woodstox JARs to the same Linux filesystem. All timed file reads access those ext4 `/tmp` copies, not `/mnt/g` or `/mnt/c`. Source copying, compilation, checksum prechecks and writing result files to Windows are outside the measured operations. File contents are verified against the recorded Windows SHA-256 hashes before each asset is benchmarked. Linux and Windows event/checksum pairs were compared after the run and all 24 matched.

The source checkout contains `pkg/stax-xml-reference` at commit `07cca8a0f53f83a47de4149f84749e4259dc9360`; the Windows machine's Maven cache supplies Woodstox 7.2.0 and StAX2 4.3.0. `setup.sh` downloads Go 1.26.0 and the latest Temurin 25 JDK. Later downloads may therefore use a different JDK patch version; the measured run used 25.0.5. Rust and cargo must already be installed. Paths in the scripts are machine-specific and should be adjusted elsewhere. They write only their `/tmp/go-xml-v2-benchmark` working directory and the benchmark result files, and do not install system packages. The WSL distribution may clear `/tmp` when stopped, so run preparation and measurement in one WSL session:

```sh
bash /mnt/g/programming/go-xml-v2/benchmark-results/linux-amd64/setup.sh && \
  bash /mnt/g/programming/go-xml-v2/benchmark-results/linux-amd64/run.sh
```

`samples.csv` contains 204 samples and `summary.csv` contains 24 rows. These are warm-cache results; filesystem differences, WSL virtualization and different compiler/JDK versions limit cross-platform conclusions.
