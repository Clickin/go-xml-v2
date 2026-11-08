#!/usr/bin/env python3
"""Summarize a run.sh results directory.

Reads raw/roundNN-{none,xmlv2}.json, checks that every reader produced the
expected events and checksum, and writes:
  summary.md   per-case medians, per-round ratios, allocations
  none.txt, xmlv2.txt   go test -bench format, one line per process
                         (the median of its timed runs), for benchstat
"""
import json, os, statistics, sys

out, arch, cpu = sys.argv[1], sys.argv[2], sys.argv[3]
SIZE = 16777038
TOKEN = (967965, 36104832)      # events, checksum for the token readers

NAMES = {
    "encoding/xml Token": "DecoderToken",
    "encoding/xml Unmarshal": "Unmarshal",
    "xmltext ReadToken": "XMLTextReadToken",
    "encoding/xml/v2 Unmarshal": "V2Unmarshal",
}

rounds = {}   # round -> build -> case -> [results]
raw = os.path.join(out, "raw")
for f in sorted(os.listdir(raw)):
    r, b = f[:-5].split("-")
    with open(os.path.join(raw, f)) as fh:
        rounds.setdefault(int(r[5:]), {})[b] = json.load(fh)["cases"]

errors = []
unmarshal = set()
for r, builds in rounds.items():
    for b, cases in builds.items():
        for name, res in cases.items():
            for x in res:
                got = (x["events"], x["checksum"])
                if "Token" in name:
                    if got != TOKEN:
                        errors.append(f"round {r} {b} {name}: {got} != {TOKEN}")
                else:
                    unmarshal.add(got)
if len(unmarshal) != 1:
    errors.append(f"Unmarshal results differ: {sorted(unmarshal)}")
if errors:
    sys.exit("checksum mismatch:\n" + "\n".join(errors))

def med(xs):
    return statistics.median(xs)

# per process median of the timed runs
proc = {}     # (build, case) -> list of (round, seconds, allocs, bytes)
for r, builds in sorted(rounds.items()):
    for b, cases in builds.items():
        for name, res in cases.items():
            proc.setdefault((b, name), []).append((
                r, med([x["seconds"] for x in res]),
                med([x["allocs"] for x in res]), med([x["allocBytes"] for x in res])))

hdr = f"goos: linux\ngoarch: {arch}\npkg: encoding/xml\ncpu: {cpu}\n"
for b in ("none", "xmlv2"):
    with open(os.path.join(out, f"{b}.txt"), "w") as fh:
        fh.write(hdr)
        for name in NAMES:
            for r, sec, allocs, nbytes in proc.get((b, name), []):
                ns = sec * 1e9
                fh.write(f"Benchmark{NAMES[name]} 1 {ns:.0f} ns/op {SIZE / sec / 1e6:.2f} MB/s "
                         f"{nbytes:.0f} B/op {allocs:.0f} allocs/op\n")

lines = []
env = open(os.path.join(out, "environment.txt")).read()
lines += ["# GOEXPERIMENT=xmlv2 reader benchmark", "", "```", env.strip(), "```", ""]
lines += [f"One 16 MiB document ({SIZE:,} bytes), file read included. Each process: "
          "warmups, then timed runs; a process counts as one sample (its median). "
          "Rounds alternate which build runs first.", ""]
lines += ["| Case | Build | Median of process medians | MiB/s | Allocs / run | Processes |",
          "|---|---|---:|---:|---:|---:|"]
for name in NAMES:
    for b in ("none", "xmlv2"):
        p = proc.get((b, name))
        if not p:
            continue
        s = med([x[1] for x in p])
        lines.append(f"| `{name}` | {'current' if b == 'none' else 'GOEXPERIMENT=xmlv2'} | "
                     f"{s * 1e3:.2f} ms | {SIZE / s / 2**20:.1f} | {med([x[2] for x in p]):,.0f} | {len(p)} |")
lines += ["", "Ratio current ÷ xmlv2 for the same case, paired by round (larger means xmlv2 is faster):", "",
          "| Case | Median ratio | Min | Max |", "|---|---:|---:|---:|"]
for name in ("encoding/xml Token", "encoding/xml Unmarshal"):
    a = dict((x[0], x[1]) for x in proc[("none", name)])
    c = dict((x[0], x[1]) for x in proc[("xmlv2", name)])
    rs = [a[r] / c[r] for r in sorted(a) if r in c]
    lines.append(f"| `{name}` | {med(rs):.3f}× | {min(rs):.3f}× | {max(rs):.3f}× |")
lines += ["", f"All readers matched the expected events and checksum ({TOKEN[0]:,} / {TOKEN[1]:,}); "
          "all Unmarshal cases decoded the same records.", ""]
with open(os.path.join(out, "summary.md"), "w") as fh:
    fh.write("\n".join(lines))
print("\n".join(lines))
