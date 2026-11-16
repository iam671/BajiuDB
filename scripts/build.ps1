param([string]$OutputFile = 'BajiuDB.exe')
$ErrorActionPreference = 'Stop'
if ($OutputFile -notmatch '^BajiuDB(?:-package)?\.exe$') { throw 'Invalid build output filename.' }
$projectDir = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectDir
try {
    $config = Get-Content -LiteralPath 'wails.json' -Raw | ConvertFrom-Json
    $package = Get-Content -LiteralPath 'frontend/package.json' -Raw | ConvertFrom-Json
    $lockVersions = @(& node -e "const lock = require('./frontend/package-lock.json'); console.log(lock.version); console.log(lock.packages[''].version)")
    if ($LASTEXITCODE -ne 0 -or $lockVersions.Count -ne 2) { throw 'Cannot read package-lock.json versions.' }
    if ($config.info.productVersion -ne $package.version -or $lockVersions[0] -ne $package.version -or $lockVersions[1] -ne $package.version) {
        throw 'Product versions differ. Keep Wails, package.json and package-lock.json consistent.'
    }
    $wailsCommand = Get-Command wails -ErrorAction SilentlyContinue
    $workspaceCLI = [IO.Path]::GetFullPath((Join-Path $projectDir '../../.local/tools/wails.exe'))
    if ($wailsCommand) {
        & $wailsCommand.Source build -o $OutputFile -trimpath -ldflags '-s -w'
    } elseif (Test-Path -LiteralPath $workspaceCLI) {
        & $workspaceCLI build -o $OutputFile -trimpath -ldflags '-s -w'
    } else {
        go run github.com/wailsapp/wails/v2/cmd/wails@v2.11.0 build -o $OutputFile -trimpath -ldflags '-s -w'
    }
    if ($LASTEXITCODE -ne 0) { throw 'Bajiu DB build failed.' }
    Get-Item -LiteralPath (Join-Path $projectDir "build\bin\$OutputFile") -ErrorAction SilentlyContinue | Select-Object FullName, Length
} finally {
    Pop-Location
}
