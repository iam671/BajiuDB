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
    npm ci --prefix frontend
    if ($LASTEXITCODE -ne 0) { throw 'Frontend install failed.' }
    npm run build --prefix frontend
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
    go run ./tools/resourcegen
    if ($LASTEXITCODE -ne 0) { throw 'Windows resource generation failed.' }
    New-Item -ItemType Directory -Force build/bin | Out-Null
    go build -tags production -trimpath -ldflags '-s -w -H windowsgui' -o "build/bin/$OutputFile" .
    if ($LASTEXITCODE -ne 0) { throw 'Bajiu DB build failed.' }
    Get-Item -LiteralPath (Join-Path $projectDir "build\bin\$OutputFile") -ErrorAction SilentlyContinue | Select-Object FullName, Length
} finally {
    Pop-Location
}
