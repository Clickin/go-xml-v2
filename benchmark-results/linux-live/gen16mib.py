#!/usr/bin/env python3
"""Write runtime-comparison-16mib.xml: a Python port of generateXmlFile in
Clickin/stax-xml packages/benchmark/runtime-matrix.mjs (07cca8a0).
Output must have SHA-256 1285dbb0d198c4c5e579faa626f7ef02b0b279b1182d37d4039f3acb5929d7d4."""
import sys

def generate(path, target=16 * 1024 * 1024):
    header = b'<?xml version="1.0" encoding="UTF-8"?>\n<root>\n'
    footer = b'</root>\n'
    total = len(header)
    parts = [header]
    i = 0
    while total + len(footer) < target:
        el = (f'  <book id="book-{i}" lang="en" code="{i % 97}">'
              f'<title>Runtime Benchmark {i}</title>'
              f'<author>Author {i % 4096}</author>'
              f'<description>Full string checksum text payload {i} with stable words and numbers.</description>'
              f'<chapter number="1">Intro {i}</chapter>'
              f'<chapter number="2">Body {i}</chapter>'
              '</book>\n').encode()
        if total + len(el) + len(footer) > target:
            break
        parts.append(el)
        total += len(el)
        i += 1
    parts.append(footer)
    with open(path, 'wb') as f:
        f.write(b''.join(parts))

if __name__ == '__main__':
    generate(sys.argv[1])
