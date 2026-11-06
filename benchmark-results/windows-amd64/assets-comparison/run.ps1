$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot/../../..").Path
$assetDir = "$root/pkg/stax-xml-reference/packages/benchmark/assets"
$jars = 'C:/Users/z1z0b/.m2/repository/com/fasterxml/woodstox/woodstox-core/7.2.0/woodstox-core-7.2.0.jar;C:/Users/z1z0b/.m2/repository/org/codehaus/woodstox/stax2-api/4.3.0/stax2-api-4.3.0.jar'
$cp = "$root/pkg/asset-java;$jars"
$assets = @('mini-sample.xml','sample.xml','complex.xml','books.xml','ptest.xml','crlf.xml','midsize.xml','large.xml')
$env:GOMAXPROCS='8'
$samples = @(); $metadata = @()
foreach ($file in $assets) {
 $path = "$assetDir/$file"; $size = (Get-Item $path).Length
 $metadata += [pscustomobject]@{asset=$file;bytes=$size;sha256=(Get-FileHash $path -Algorithm SHA256).Hash}
 $runs = if ($size -gt 20MB) { 5 } else { 9 }; $warmups=if ($size -gt 20MB) { 2 } else { 4 }; $reps=if ($size -lt 1MB) { 100 } else { 1 }
 $expected = $null
 foreach ($impl in @('go','woodstox','quick-xml')) {
  $out = switch ($impl) {
   'go' { & "$root/pkg/asset-go.exe" $path $warmups $runs $reps }
   'woodstox' { & java -Xms512m -Xmx512m -cp $cp AssetBench $path $warmups $runs $reps }
   'quick-xml' { & "$root/pkg/asset-target/release/xml-assets-comparison.exe" $path $warmups $runs $reps }
  }
  if ($LASTEXITCODE -ne 0) { throw "$file $impl failed" }
  $values = ($out | ConvertFrom-Json).samples
  if ($values.Count -ne $runs) { throw "$file $impl sample count" }
  foreach ($v in $values) {
   $identity = "$($v.events)/$($v.checksum)"
   if ($null -eq $expected) { $expected = $identity }
   if ($expected -ne $identity) { throw "$file ${impl}: expected $expected, got $identity" }
   $samples += [pscustomobject]@{asset=$file;bytes=$size;implementation=$impl;events=$v.events;checksum=$v.checksum;seconds=$v.seconds}
  }
 }
 Write-Host "$file parity $expected"
}
$metadata | Export-Csv "$PSScriptRoot/assets.csv" -NoTypeInformation
$samples | Export-Csv "$PSScriptRoot/samples.csv" -NoTypeInformation
$samples | Group-Object asset,implementation | ForEach-Object {
 $g=$_.Group; $s=@($g.seconds | Sort-Object); $mid=[int][Math]::Floor($s.Count/2); $median=[double]$s[$mid]
 [pscustomobject]@{asset=$g[0].asset;bytes=$g[0].bytes;implementation=$g[0].implementation;samples=$s.Count;median_ms=$median*1000;min_ms=[double]$s[0]*1000;max_ms=[double]$s[-1]*1000;median_MiBs=$g[0].bytes/1MB/$median;events=$g[0].events;checksum=$g[0].checksum}
} | Export-Csv "$PSScriptRoot/summary.csv" -NoTypeInformation
Import-Csv "$PSScriptRoot/summary.csv" | Format-Table asset,implementation,median_ms,median_MiBs,events -AutoSize
