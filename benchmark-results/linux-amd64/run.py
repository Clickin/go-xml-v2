import csv
import hashlib
import json
import os
from pathlib import Path
import platform
import statistics
import subprocess

root = Path('/mnt/g/programming/go-xml-v2')
work = Path('/tmp/go-xml-v2-benchmark')
out = root / 'benchmark-results/linux-amd64'
windows = root / 'benchmark-results/windows-amd64/assets-comparison'
with (windows / 'assets.csv').open(newline='', encoding='utf-8-sig') as f:
    assets = list(csv.DictReader(f))

def version(*command):
    return subprocess.run(command, text=True, capture_output=True, check=True).stdout.strip() or subprocess.run(command, text=True, capture_output=True, check=True).stderr.strip()

env = {
    'kernel': platform.platform(), 'machine': platform.machine(),
    'cpu': next((x.split(':', 1)[1].strip() for x in Path('/proc/cpuinfo').read_text().splitlines() if x.startswith('model name')), ''),
    'go': version(str(work / 'go-src/bin/go'), 'version'),
    'rust': version('rustc', '--version'),
    'java': version(str(work / 'jdk/bin/java'), '-version'),
    'go_source_revision': 'bd5149100704087efd1edf06bfb7d6956aa636ac',
    'goexperiment': 'xmlv2', 'goamd64': 'v1', 'gomaxprocs': 8,
    'woodstox': '7.2.0', 'quick_xml': '0.42.0',
    'filesystem': 'WSL ext4 /tmp, warm page cache',
}
(out / 'environment.json').write_text(json.dumps(env, indent=2) + '\n', encoding='utf-8')
rows = []
for asset in assets:
    path = work / 'assets' / asset['asset']
    size = path.stat().st_size
    digest = hashlib.sha256(path.read_bytes()).hexdigest().upper()
    if size != int(asset['bytes']) or digest != asset['sha256']:
        raise RuntimeError(f'Windows asset mismatch: {path}')
    runs, warmups = (5, 2) if size > 20 * 1024**2 else (9, 4)
    reps = 100 if size < 1024**2 else 1
    args = [str(path), str(warmups), str(runs), str(reps)]
    commands = {
        'go': [str(work / 'asset-go'), *args],
        'woodstox': [str(work / 'jdk/bin/java'), '-Xms512m', '-Xmx512m', '-cp', f'{work}/java:{work}/woodstox.jar:{work}/stax2.jar', 'AssetBench', *args],
        'quick-xml': [str(work / 'cargo-target/release/xml-assets-comparison'), *args],
    }
    expected = None
    for name, command in commands.items():
        data = json.loads(subprocess.run(command, text=True, capture_output=True, check=True).stdout)['samples']
        if len(data) != runs:
            raise RuntimeError(f'{path.name}: wrong sample count')
        for sample in data:
            identity = (sample['events'], sample['checksum'])
            if expected is None:
                expected = identity
            if identity != expected:
                raise RuntimeError(f'{path.name} {name}: expected {expected}, got {identity}')
            rows.append(dict(asset=path.name, bytes=size, implementation=name, **sample))
    print(path.name, 'parity', expected, flush=True)

with (out / 'samples.csv').open('w', newline='', encoding='utf-8') as f:
    writer = csv.DictWriter(f, fieldnames=['asset', 'bytes', 'implementation', 'events', 'checksum', 'seconds'])
    writer.writeheader(); writer.writerows(rows)
summary = []
for asset in assets:
    for name in ('go', 'woodstox', 'quick-xml'):
        group = [r for r in rows if r['asset'] == asset['asset'] and r['implementation'] == name]
        seconds = [r['seconds'] for r in group]
        median = statistics.median(seconds)
        summary.append(dict(asset=asset['asset'], bytes=group[0]['bytes'], implementation=name, samples=len(group),
                            median_ms=median*1000, min_ms=min(seconds)*1000, max_ms=max(seconds)*1000,
                            median_MiBs=group[0]['bytes']/1024**2/median, events=group[0]['events'], checksum=group[0]['checksum']))
with (out / 'summary.csv').open('w', newline='', encoding='utf-8') as f:
    writer = csv.DictWriter(f, fieldnames=summary[0].keys()); writer.writeheader(); writer.writerows(summary)
