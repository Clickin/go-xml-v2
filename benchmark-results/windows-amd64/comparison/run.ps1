$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot/../../..").Path
$data = "$root/pkg/bench-catalog-16mib.xml"
$woodstox = 'C:/Users/z1z0b/.m2/repository/com/fasterxml/woodstox/woodstox-core/7.2.0/woodstox-core-7.2.0.jar'
$stax = 'C:/Users/z1z0b/.m2/repository/org/codehaus/woodstox/stax2-api/4.3.0/stax2-api-4.3.0.jar'
$classpath = "$root/pkg/comparison-java;$woodstox;$stax"
$env:GOMAXPROCS = '8'
$lines = @('implementation,iteration,ns,checksum,process')
$orders = @(@('go', 'woodstox', 'quick-xml'), @('woodstox', 'quick-xml', 'go'), @('quick-xml', 'go', 'woodstox'))
for ($run = 0; $run -lt 3; $run++) {
  foreach ($impl in $orders[$run]) {
    $output = switch ($impl) {
      'go' { & "$root/pkg/comparison-go.exe" $data }
      'woodstox' { & java -Xms512m -Xmx512m -cp $classpath WoodstoxBench $data }
      'quick-xml' { & "$root/pkg/comparison-target/release/xml-comparison.exe" $data }
    }
    if ($LASTEXITCODE -ne 0) { throw "$impl failed" }
    $lines += @($output | ForEach-Object { "$_,$run" })
  }
}
$lines | Set-Content "$PSScriptRoot/samples.csv"
$results = @($lines | ConvertFrom-Csv)
if ($results.Count -ne 90 -or @($results.checksum | Select-Object -Unique).Count -ne 1) { throw 'Unexpected sample count or mismatched checksum' }
$size = (Get-Item $data).Length
$results | Group-Object implementation | ForEach-Object {
  $values = @($_.Group.ns | ForEach-Object { [double]$_ } | Sort-Object)
  $median = ($values[14] + $values[15]) / 2
  [pscustomobject]@{ implementation=$_.Name; samples=$values.Count; median_ms=$median/1e6; min_ms=$values[0]/1e6; max_ms=$values[29]/1e6; median_MBps=$size/($median/1e9)/1e6; checksum=$_.Group[0].checksum }
} | Export-Csv "$PSScriptRoot/summary.csv" -NoTypeInformation
Import-Csv "$PSScriptRoot/summary.csv" | Format-Table -AutoSize
