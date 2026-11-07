# Cross-Language Reader Benchmark

Generated: 2026-11-06T05:01:09.070Z

The same UTF-8 XML file is read and parsed through public pull-reader APIs. Each timed run includes file I/O and materializes start/end elements, non-whitespace text, and every attribute name/value into the same checksum.

| Reader | Median throughput | Median time | Events | Checksum |
| --- | ---: | ---: | ---: | ---: |
| stax-xml | 98.9 MiB/s | 161.74 ms | 967965 | 36104832 |
| woodstox | 350.0 MiB/s | 45.72 ms | 967965 | 36104832 |
| quick-xml | 572.6 MiB/s | 27.94 ms | 967965 | 36104832 |
