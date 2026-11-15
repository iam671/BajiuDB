$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectDir
try {
    $wailsCommand = Get-Command wails -ErrorAction SilentlyContinue
    if ($wailsCommand) {
        & $wailsCommand.Source build -trimpath -ldflags '-s -w'
    } else {
        go run github.com/wailsapp/wails/v2/cmd/wails@v2.11.0 build -trimpath -ldflags '-s -w'
    }
    if ($LASTEXITCODE -ne 0) { throw 'Bajiu DB build failed.' }
    Get-Item -LiteralPath (Join-Path $projectDir 'build\bin\BajiuDB.exe') -ErrorAction SilentlyContinue | Select-Object FullName, Length
} finally {
    Pop-Location
}
