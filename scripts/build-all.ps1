# Build Knitnox for every supported OS/architecture into dist\ (Windows PowerShell 5.1+).
# Usage: .\scripts\build-all.ps1            or    .\scripts\build-all.ps1 -Version 1.2.0
param([string]$Version = "dev")
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

$out = "dist"
if (Test-Path $out) { Remove-Item -Recurse -Force $out }
New-Item -ItemType Directory $out | Out-Null

$targets = @("linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64")
foreach ($t in $targets) {
    $os, $arch = $t.Split("/")
    $ext = ""; if ($os -eq "windows") { $ext = ".exe" }
    $name = "knitnox-$Version-$os-$arch"
    Write-Host "building $name"
    $env:CGO_ENABLED = "0"; $env:GOOS = $os; $env:GOARCH = $arch
    go build -trimpath -ldflags="-s -w" -o "$out\$name\knitnox$ext" ./cmd/knitnox
    if ($LASTEXITCODE -ne 0) { throw "build failed for $t" }
    Copy-Item README.md "$out\$name\"
    Compress-Archive -Path "$out\$name" -DestinationPath "$out\$name.zip"
    Remove-Item -Recurse -Force "$out\$name"
}
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue

Get-ChildItem $out -File | Get-FileHash -Algorithm SHA256 | ForEach-Object { "$($_.Hash.ToLower())  $(Split-Path $_.Path -Leaf)" } | Set-Content -Encoding ascii "$out\SHA256SUMS.txt"
Get-ChildItem $out
