param(
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA "tsv")
)

$ErrorActionPreference = "Stop"
$parent = Split-Path -Parent $InstallDir
if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
    throw "Installation parent does not exist: $parent"
}
if (-not (Test-Path -LiteralPath $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}
$source = Join-Path $PSScriptRoot "tsv-windows-amd64.exe"
if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
    throw "Build tsv-windows-amd64.exe before installing."
}
Copy-Item -LiteralPath $source -Destination (Join-Path $InstallDir "tsv.exe") -Force
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$entries = @($userPath -split ";" | Where-Object { $_ })
if (-not ($entries | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) {
    $newPath = (@($entries) + $InstallDir) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
}
if (-not (($env:Path -split ";") | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) {
    $env:Path = "$env:Path;$InstallDir"
}
"Installed: $(Join-Path $InstallDir 'tsv.exe')"
"Open a new terminal and run: tsv"
