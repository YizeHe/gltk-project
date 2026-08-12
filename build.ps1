# Build GrokLangToolKit (gltk.exe)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

Write-Host "==> go test ./..."
go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> go build -o gltk.exe ./cmd/gltk"
go build -o gltk.exe ./cmd/gltk
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> ./gltk.exe version"
./gltk.exe version

Write-Host "Build OK: $PSScriptRoot\gltk.exe"
