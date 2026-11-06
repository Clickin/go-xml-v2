$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot/../..").Path
$env:GOROOT = $root
$env:GOCACHE = "$root/pkg/bench-cache"
$env:GOPATH = "$root/pkg/bench-gopath"
$env:GOTOOLCHAIN = 'local'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:GOAMD64 = 'v1'
$env:CGO_ENABLED = '0'
$env:GOEXPERIMENT = 'xmlv2'
$env:GOMAXPROCS = '8'
$go = "$root/bin/go.exe"
# Reproducible synthetic corpus; not the external runtime-comparison fixture.
$entry = '<book id="1" lang="en" code="ABC"><title>Example title</title><author>Author</author><description>Text &amp; more text.</description><chapter number="1">Chapter text.</chapter></book>'
$count = [int][Math]::Floor((16MB - 19) / $entry.Length)
$corpus = '<catalog>' + ($entry * $count) + '</catalog>'
$env:XMLFILE = "$root/pkg/bench-catalog-16mib.xml"
[IO.File]::WriteAllText($env:XMLFILE, $corpus, [Text.UTF8Encoding]::new($false))
@(
    "commit: $(git -c "safe.directory=$($root.Replace('\', '/'))" -C $root rev-parse HEAD)"
    "date: $(Get-Date -Format o)"
    (& $go version)
    (& $go env GOOS GOARCH GOAMD64 GOEXPERIMENT CGO_ENABLED GOROOT)
    'GOMAXPROCS: 8'
    "corpus: synthetic catalog; books=$count; bytes=$($corpus.Length)"
    "corpus SHA256: $((Get-FileHash $env:XMLFILE -Algorithm SHA256).Hash)"
) | Set-Content "$PSScriptRoot/environment.txt"
& $go test encoding/xml/... -count=1 2>&1 | Tee-Object "$PSScriptRoot/tests.txt"
if ($LASTEXITCODE -ne 0) { throw 'XML tests failed' }
& $go test encoding/xml/... '-run=^$' '-bench=.' -benchmem -benchtime=1s -count=3 -timeout=30m 2>&1 | Tee-Object "$PSScriptRoot/bench.txt"
if ($LASTEXITCODE -ne 0) { throw 'XML benchmarks failed' }
