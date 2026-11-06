"""Same-session A/B of the Go asset harness before and after an xmltext change,
next to Woodstox and quick-xml, with and without the file read.

Every implementation runs the stax-xml assets in two modes: "file" (read the
file and parse, as the asset comparison does) and "mem" (read once, time only
the parse and checksum). Each round starts one process per asset, mode and
implementation, rotating the implementation order between rounds.

Usage:
  python3 run.py --assets DIR --go-base EXE --go-new EXE --quick-xml EXE \
      --java JAVA --java-cp CLASSPATH --out DIR [--rounds 3]

It aborts unless the assets match ../windows-amd64/assets-comparison/assets.csv
and every implementation gives the same events and checksum for an asset.
"""

import argparse
import csv
import hashlib
import json
import os
import statistics
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
ASSETS_CSV = HERE.parent / "windows-amd64" / "assets-comparison" / "assets.csv"
IMPLS = ["go-base", "go-new", "woodstox", "quick-xml"]
MODES = ["file", "mem"]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--assets", required=True)
    ap.add_argument("--go-base", required=True)
    ap.add_argument("--go-new", required=True)
    ap.add_argument("--quick-xml", required=True)
    ap.add_argument("--java", default="java")
    ap.add_argument("--java-cp", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--rounds", type=int, default=3)
    args = ap.parse_args()
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)

    with ASSETS_CSV.open(newline="", encoding="utf-8-sig") as f:
        assets = list(csv.DictReader(f))
    for a in assets:
        p = Path(args.assets) / a["asset"]
        data = p.read_bytes()
        if len(data) != int(a["bytes"]) or hashlib.sha256(data).hexdigest().upper() != a["sha256"]:
            sys.exit(f"{p} does not match assets.csv (expected the committed LF bytes)")

    def command(impl, path, w, n, reps, mode):
        a = [path, str(w), str(n), str(reps), mode]
        if impl == "go-base":
            return [args.go_base, *a]
        if impl == "go-new":
            return [args.go_new, *a]
        if impl == "quick-xml":
            return [args.quick_xml, *a]
        return [args.java, "-Xms512m", "-Xmx512m", "-cp", args.java_cp, "AssetBench", *a]

    rows = []
    expected = {}
    for rnd in range(args.rounds):
        order = IMPLS[rnd % len(IMPLS):] + IMPLS[:rnd % len(IMPLS)]
        for a in assets:
            size = int(a["bytes"])
            path = str(Path(args.assets) / a["asset"])
            big = size > 20 * 1024**2
            w, n = (2, 5) if big else (4, 9)
            for mode in MODES:
                reps = 1 if size >= 1024**2 else (100 if mode == "file" else 1000)
                for impl in order:
                    res = subprocess.run(command(impl, path, w, n, reps, mode),
                                         capture_output=True, text=True)
                    if res.returncode != 0:
                        sys.exit(f"{a['asset']} {impl} {mode} failed:\n{res.stderr}")
                    for s in json.loads(res.stdout)["samples"]:
                        ident = (s["events"], s["checksum"])
                        if expected.setdefault(a["asset"], ident) != ident:
                            sys.exit(f"{a['asset']} {impl} {mode}: expected {expected[a['asset']]}, got {ident}")
                        rows.append(dict(round=rnd, asset=a["asset"], bytes=size, implementation=impl,
                                         mode=mode, events=s["events"], checksum=s["checksum"],
                                         seconds=s["seconds"]))
            print(f"round {rnd} {a['asset']} parity {expected[a['asset']]}", flush=True)

    with (out / "samples.csv").open("w", newline="", encoding="utf-8") as f:
        wr = csv.DictWriter(f, fieldnames=list(rows[0].keys()))
        wr.writeheader()
        wr.writerows(rows)

    med = {}
    summary = []
    for a in assets:
        for mode in MODES:
            for impl in IMPLS:
                sec = [r["seconds"] for r in rows
                       if r["asset"] == a["asset"] and r["mode"] == mode and r["implementation"] == impl]
                m = statistics.median(sec)
                med[(a["asset"], mode, impl)] = m
                summary.append(dict(asset=a["asset"], bytes=a["bytes"], mode=mode, implementation=impl,
                                    samples=len(sec), median_us=m * 1e6, min_us=min(sec) * 1e6,
                                    max_us=max(sec) * 1e6, median_MiBs=int(a["bytes"]) / 1024**2 / m))
    with (out / "summary.csv").open("w", newline="", encoding="utf-8") as f:
        wr = csv.DictWriter(f, fieldnames=list(summary[0].keys()))
        wr.writeheader()
        wr.writerows(summary)

    # Ratios are throughput ratios: larger means the first is faster.
    lines = ["| asset | mode | new÷base | base÷Woodstox | new÷Woodstox | base÷quick-xml | new÷quick-xml |",
             "| --- | --- | ---: | ---: | ---: | ---: | ---: |"]
    for a in assets:
        for mode in MODES:
            t = {i: med[(a["asset"], mode, i)] for i in IMPLS}
            lines.append(f"| {a['asset']} | {mode} | {t['go-base'] / t['go-new']:.2f} | "
                         f"{t['woodstox'] / t['go-base']:.2f} | {t['woodstox'] / t['go-new']:.2f} | "
                         f"{t['quick-xml'] / t['go-base']:.2f} | {t['quick-xml'] / t['go-new']:.2f} |")
    (out / "ratios.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
