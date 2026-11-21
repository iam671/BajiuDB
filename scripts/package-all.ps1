param([switch]$SkipWindowsBuild, [switch]$SkipCrossBuild)
$ErrorActionPreference = 'Stop'
$projectDir = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
Push-Location -LiteralPath $projectDir
$priorEnvironment = @{}
foreach ($key in @('GOOS', 'GOARCH', 'CGO_ENABLED')) {
    $priorEnvironment[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
    [Environment]::SetEnvironmentVariable($key, $null, 'Process')
}
try {
    if (-not $IsWindows -and $PSVersionTable.PSEdition -ne 'Desktop') {
        throw 'This orchestrator requires Windows; package-unix.py can package prebuilt binaries on other hosts.'
    }
    if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'Python 3.11+ is required for Unix archives.' }
    & python -c 'import sys; assert sys.version_info >= (3, 11), "Python 3.11+ is required"'
    if ($LASTEXITCODE -ne 0) { throw 'Python version check failed.' }
    if (-not $SkipWindowsBuild) { & (Join-Path $PSScriptRoot 'package.ps1') }
    elseif (-not (Test-Path -LiteralPath 'frontend/dist/index.html')) { throw 'Build the frontend before packaging cross-platform binaries.' }
    $binaryDir = Join-Path $projectDir '.local/cross-build'
    New-Item -ItemType Directory -Path $binaryDir -Force | Out-Null
    foreach ($target in @('linux/amd64', 'linux/arm64', 'darwin/amd64', 'darwin/arm64')) {
        $targetOS, $targetArch = $target.Split('/')
        $binary = Join-Path $binaryDir "BajiuDB-$targetOS-$targetArch"
        if (-not $SkipCrossBuild) {
            $env:GOOS = $targetOS
            $env:GOARCH = $targetArch
            $env:CGO_ENABLED = '0'
            & go build -tags 'production,purego' -trimpath -ldflags '-s -w' -o $binary .
            if ($LASTEXITCODE -ne 0) { throw "Cross compilation failed: $target" }
        }
        & python (Join-Path $PSScriptRoot 'package-unix.py') --os $targetOS --arch $targetArch --binary $binary
        if ($LASTEXITCODE -ne 0) { throw "Packaging failed: $target" }
    }
} finally {
    foreach ($key in $priorEnvironment.Keys) { [Environment]::SetEnvironmentVariable($key, $priorEnvironment[$key], 'Process') }
    Pop-Location
}
