$ErrorActionPreference = 'Stop'
$package = ''
$lines = @(Get-Content "$PSScriptRoot/bench.txt")
# A restarted run may place the completed xmltext package in a separate file.
if (-not ($lines -match '^ok\s+encoding/xml/xmltext\s') -and (Test-Path "$PSScriptRoot/xmltext.txt")) {
    $start = [Array]::IndexOf($lines, 'pkg: encoding/xml/xmltext')
    if ($start -ge 2) { $lines = @($lines[0..($start - 3)]) }
    $lines += @(Get-Content "$PSScriptRoot/xmltext.txt")
}
if (-not ($lines -match '^ok\s+encoding/xml/xmltext\s')) { throw 'xmltext benchmarks have not completed' }
$samples = foreach ($line in $lines) {
    if ($line -match '^pkg: (.+)$') { $package = $Matches[1] }
    if ($line -match '^(Benchmark\S+)\s+\d+\s+([\d.]+) ns/op\s+(?:([\d.]+) MB/s\s+)?([\d.]+) B/op\s+([\d.]+) allocs/op') {
        [pscustomobject]@{ Package=$package; Name=$Matches[1]; Ns=[double]$Matches[2]; MBps=$(if ($Matches[3]) { [double]$Matches[3] } else { $null }); Bytes=[double]$Matches[4]; Allocs=[double]$Matches[5] }
    }
}
function Median($values) { $sorted = @($values | Sort-Object); return $sorted[[int][Math]::Floor($sorted.Count / 2)] }
$rows = $samples | Group-Object Package,Name | ForEach-Object {
    $g = $_.Group
    if ($g.Count -ne 3) { throw "Expected 3 samples: $($_.Name), got $($g.Count)" }
    [pscustomobject]@{
        Package=$g[0].Package; Benchmark=$g[0].Name; Samples=$g.Count
        MedianNs=(Median $g.Ns); MinNs=($g.Ns | Measure-Object -Minimum).Minimum; MaxNs=($g.Ns | Measure-Object -Maximum).Maximum
        MedianMBps=$(if ($null -ne $g[0].MBps) { Median $g.MBps } else { $null }); MedianBytes=(Median $g.Bytes); MedianAllocs=(Median $g.Allocs)
    }
}
$rows | Export-Csv "$PSScriptRoot/summary.csv" -NoTypeInformation
$rows | Where-Object { $_.Package -eq 'encoding/xml/v2' -or $_.Benchmark -match '^Benchmark(Corpus|Small|.*V1OnXMLText)' } | Format-Table -AutoSize
"Benchmark cases: $($rows.Count); samples: $(@($samples).Count)"
