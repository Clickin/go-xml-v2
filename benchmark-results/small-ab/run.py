"""Same-session A/B of the Go asset harness before and after an xmltext change,
next to Woodstox and quick-xml, with and without the file read.

Every implementation runs the stax-xml assets in two modes: "file" (read the
file and parse, as the asset comparison does) and "mem" (read once, time only
the parse and checksum). Each round starts one process per asset, mode and
implementation, rotating the implementation order between rounds.

Usage:
  python3 run.py --assets DIR --go-base EXE --go-new EXE --quick-xml EXE \
      --java JAVA --java-cp CLASSPATH --out DIR [--rounds 9] [--cpus auto|none|0,2,4]

--cpus pins this process, and so every process it starts, to the given logical
CPUs. "auto" picks one logical CPU per performance core: on Windows the cores
of the highest EfficiencyClass, on Linux /sys/devices/cpu_core/cpus when the
kernel exposes hybrid core types. GOMAXPROCS is then set to the number of
pinned CPUs. Inside WSL this pins virtual CPUs only; the host still schedules
them.

It aborts unless the assets match ../windows-amd64/assets-comparison/assets.csv
and every implementation gives the same events and checksum for an asset.
"""

import argparse
import ctypes
import platform
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


def windows_performance_cpus():
    """One logical processor index per core of the highest EfficiencyClass."""
    k = ctypes.WinDLL("kernel32", use_last_error=True)
    n = ctypes.c_ulong(0)
    k.GetSystemCpuSetInformation(None, 0, ctypes.byref(n), None, 0)
    buf = ctypes.create_string_buffer(n.value)
    if not k.GetSystemCpuSetInformation(buf, n, ctypes.byref(n), None, 0):
        raise OSError(ctypes.get_last_error(), "GetSystemCpuSetInformation")
    raw, off, sets = buf.raw, 0, []
    while off < n.value:
        size = int.from_bytes(raw[off:off + 4], "little")
        group = int.from_bytes(raw[off + 12:off + 14], "little")
        lp, core, eff = raw[off + 14], raw[off + 15], raw[off + 18]
        sets.append((group, lp, core, eff))
        off += size
    if any(g != 0 for g, *_ in sets):
        raise RuntimeError("more than one processor group; pass --cpus")
    top = max(e for *_, e in sets)
    cpus, seen = [], set()
    for _, lp, core, eff in sorted(sets, key=lambda t: t[1]):
        if eff == top and core not in seen:
            seen.add(core)
            cpus.append(lp)
    return cpus


def linux_performance_cpus():
    p = Path("/sys/devices/cpu_core/cpus")
    if not p.exists():
        raise RuntimeError("no /sys/devices/cpu_core/cpus (hybrid core types not exposed); pass --cpus")
    cpus = []
    for part in p.read_text().strip().split(","):
        lo, _, hi = part.partition("-")
        cpus += range(int(lo), int(hi or lo) + 1)
    # one logical CPU per core
    seen, out = set(), []
    for c in cpus:
        sib = Path(f"/sys/devices/system/cpu/cpu{c}/topology/thread_siblings_list").read_text().strip()
        if sib not in seen:
            seen.add(sib)
            out.append(c)
    return out


def pin(spec):
    """Pins this process (inherited by children) and returns the CPU list, or None."""
    if spec == "none":
        return None
    windows = platform.system() == "Windows"
    if spec == "auto":
        cpus = windows_performance_cpus() if windows else linux_performance_cpus()
    else:
        cpus = [int(c) for c in spec.split(",")]
    if windows:
        mask = sum(1 << c for c in cpus)
        k = ctypes.WinDLL("kernel32", use_last_error=True)
        k.GetCurrentProcess.restype = ctypes.c_void_p
        if not k.SetProcessAffinityMask(ctypes.c_void_p(k.GetCurrentProcess()), ctypes.c_size_t(mask)):
            raise OSError(ctypes.get_last_error(), "SetProcessAffinityMask")
    else:
        os.sched_setaffinity(0, cpus)
    os.environ["GOMAXPROCS"] = str(len(cpus))
    return cpus


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--assets", required=True)
    ap.add_argument("--go-base", required=True)
    ap.add_argument("--go-new", required=True)
    ap.add_argument("--quick-xml", required=True)
    ap.add_argument("--java", default="java")
    ap.add_argument("--java-cp", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--rounds", type=int, default=9)
    ap.add_argument("--cpus", default="auto")
    args = ap.parse_args()
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    cpus = pin(args.cpus)
    print(f"pinned to CPUs {cpus}" if cpus else "not pinned", flush=True)
    (out / "environment.json").write_text(json.dumps(dict(
        platform=platform.platform(), machine=platform.machine(), cpus=cpus,
        gomaxprocs=os.environ.get("GOMAXPROCS"), rounds=args.rounds), indent=2) + "\n", encoding="utf-8")

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

    # Ratios are throughput ratios: larger means the first is faster. Each cell
    # is the median of the per-round ratios, then their range (each round is
    # one process per implementation, so a round is one independent sample).
    def per_round(asset, mode, impl):
        return [statistics.median(r["seconds"] for r in rows if r["asset"] == asset and r["mode"] == mode
                                  and r["implementation"] == impl and r["round"] == rnd)
                for rnd in range(args.rounds)]

    pairs = [("new÷base", "go-new", "go-base"), ("base÷Woodstox", "go-base", "woodstox"),
             ("new÷Woodstox", "go-new", "woodstox"), ("base÷quick-xml", "go-base", "quick-xml"),
             ("new÷quick-xml", "go-new", "quick-xml")]
    lines = ["| asset | mode | " + " | ".join(p[0] for p in pairs) + " |",
             "| --- | --- |" + " ---: |" * len(pairs)]
    for a in assets:
        for mode in MODES:
            rr = {i: per_round(a["asset"], mode, i) for i in IMPLS}
            cells = []
            for _, x, y in pairs:
                ratios = [ty / tx for tx, ty in zip(rr[x], rr[y])]
                cells.append(f"{statistics.median(ratios):.2f} "
                             f"({min(ratios):.2f}–{max(ratios):.2f})")
            lines.append(f"| {a['asset']} | {mode} | " + " | ".join(cells) + " |")
    (out / "ratios.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("\n".join(lines))

if __name__ == "__main__":
    main()
