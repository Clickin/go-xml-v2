# GOEXPERIMENT=xmlv2 reader benchmark

```
date_utc: 20261108T070555Z
kernel: 7.0.0-30-generic
model name	: 13th Gen Intel(R) Core(TM) i5-13600K
pinned_cpus: 0-11 (performance cores (/sys/devices/cpu_core/cpus))
governor: performance (was powersave)
intel_pstate no_turbo: 0 (was 0)
MemTotal:       65549096 kB
virtualization: none
unknown
go: go version go1.28-devel_c759c45a38 Sun Nov 8 08:06:42 2026 +0900 linux/amd64
xml-v2-poc: c759c45a388b282e324d7cd9404e5a078e490d34
fixture: 1285dbb0d198c4c5e579faa626f7ef02b0b279b1182d37d4039f3acb5929d7d4
rounds: 20, warmups: 3, runs: 10
PRETTY_NAME="Ubuntu 26.04.1 LTS"
```

One 16 MiB document (16,777,038 bytes), file read included. Each process: warmups, then timed runs; a process counts as one sample (its median). Rounds alternate which build runs first.

| Case | Build | Median of process medians | MiB/s | Allocs / run | Processes |
|---|---|---:|---:|---:|---:|
| `encoding/xml Token` | current | 215.07 ms | 74.4 | 3,125,804 | 20 |
| `encoding/xml Token` | GOEXPERIMENT=xmlv2 | 95.58 ms | 167.4 | 1,360,842 | 20 |
| `encoding/xml Unmarshal` | current | 314.64 ms | 50.9 | 4,600,375 | 20 |
| `encoding/xml Unmarshal` | GOEXPERIMENT=xmlv2 | 130.61 ms | 122.5 | 1,024,986 | 20 |
| `xmltext ReadToken` | GOEXPERIMENT=xmlv2 | 36.96 ms | 432.9 | 161 | 20 |
| `encoding/xml/v2 Unmarshal` | GOEXPERIMENT=xmlv2 | 77.02 ms | 207.7 | 563,589 | 20 |

Ratio current ÷ xmlv2 for the same case, paired by round (larger means xmlv2 is faster):

| Case | Median ratio | Min | Max |
|---|---:|---:|---:|
| `encoding/xml Token` | 2.249× | 2.237× | 2.260× |
| `encoding/xml Unmarshal` | 2.409× | 2.390× | 2.421× |

All readers matched the expected events and checksum (967,965 / 36,104,832); all Unmarshal cases decoded the same records.
