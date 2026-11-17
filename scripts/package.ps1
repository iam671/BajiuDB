$ErrorActionPreference = 'Stop'
$projectDir = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$distDir = Join-Path $projectDir 'dist'
$workDir = Join-Path $projectDir '.local/package-staging'

# Never accept deletion targets outside this application's named output roots.
function Remove-Output([string]$Path, [string]$Root) {
    $target = [IO.Path]::GetFullPath($Path)
    $boundary = [IO.Path]::GetFullPath($Root).TrimEnd('\') + '\'
    if (-not $target.StartsWith($boundary, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to clean outside output directory: $target"
    }
    if (Test-Path -LiteralPath $target) { Remove-Item -LiteralPath $target -Recurse -Force }
}

& (Join-Path $PSScriptRoot 'build.ps1') -OutputFile 'BajiuDB-package.exe'
& node (Join-Path $PSScriptRoot 'collect-licenses.cjs')
if ($LASTEXITCODE -ne 0) { throw 'License collection failed.' }
$version = (Get-Content -LiteralPath (Join-Path $projectDir 'wails.json') -Raw | ConvertFrom-Json).info.productVersion
if ($version -notmatch '^\d+\.\d+\.\d+$') { throw 'Invalid product version.' }
$name = "BajiuDB-$version-windows-x64"
$stage = Join-Path $workDir ([Guid]::NewGuid().ToString('N'))
$release = Join-Path $stage $name
New-Item -ItemType Directory -Path $release -Force | Out-Null
try {
    $exe = Join-Path $projectDir 'build/bin/BajiuDB-package.exe'
    Copy-Item -LiteralPath $exe -Destination (Join-Path $release 'BajiuDB.exe')
    foreach ($file in @('README.md', 'VALIDATION.md', 'design-qa.md', 'THIRD_PARTY_NOTICES.md', 'licenses', 'third_party_sources', 'tunnel')) {
        Copy-Item -LiteralPath (Join-Path $projectDir $file) -Destination $release -Recurse
    }
    $hash = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $release 'SHA256SUMS.txt'), "$hash  BajiuDB.exe`n", [Text.UTF8Encoding]::new($false))
    $zip = Join-Path $stage "$name.zip"
    Compress-Archive -LiteralPath $release -DestinationPath $zip -CompressionLevel Optimal
    $archive = [IO.Compression.ZipFile]::OpenRead($zip)
    try {
        $entry = $archive.GetEntry("$name/BajiuDB.exe")
        if (-not $entry) { throw 'Executable missing from release ZIP.' }
        $stream = $entry.Open()
        $sha = [Security.Cryptography.SHA256]::Create()
        try { $packedHash = ([BitConverter]::ToString($sha.ComputeHash($stream))).Replace('-', '').ToLowerInvariant() }
        finally { $sha.Dispose(); $stream.Dispose() }
        if ($hash -ne $packedHash) { throw 'Release ZIP executable hash mismatch.' }
    } finally { $archive.Dispose() }
    $zipHash = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $stage "$name.sha256"), "$zipHash  $name.zip`n", [Text.UTF8Encoding]::new($false))

    # Fixed names replace the previous package of this version; other versions remain intact.
    New-Item -ItemType Directory -Path $distDir -Force | Out-Null
    $previousExe = Join-Path $distDir "$name/BajiuDB.exe"
    $inUse = @(Get-Process -Name 'BajiuDB' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $previousExe })
    if ($inUse.Count -gt 0 -and (Test-Path -LiteralPath $previousExe)) {
        # A running Windows image can be renamed but cannot be deleted. Keep the
        # current session alive while publishing the already-validated new package.
        $heldRoot = [IO.Path]::GetFullPath((Join-Path $projectDir '.local/package-in-use'))
        $heldDir = [IO.Path]::GetFullPath((Join-Path $heldRoot ([Guid]::NewGuid().ToString('N'))))
        if (-not $heldDir.StartsWith($heldRoot + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Invalid in-use package directory.' }
        New-Item -ItemType Directory -Path $heldDir -Force | Out-Null
        Move-Item -LiteralPath $previousExe -Destination (Join-Path $heldDir 'BajiuDB.exe')
        Write-Warning 'The current app session remains open. Close it before opening the updated release.'
    }
    foreach ($output in @($name, "$name.zip", "$name.sha256")) {
        Remove-Output (Join-Path $distDir $output) $distDir
        Move-Item -LiteralPath (Join-Path $stage $output) -Destination $distDir
    }
    $canonical = Join-Path $projectDir 'build/bin/BajiuDB.exe'
    $running = @(Get-Process -Name 'BajiuDB' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $canonical })
    if ($running.Count -eq 0) {
        Copy-Item -LiteralPath $exe -Destination $canonical -Force
    } else {
        Write-Warning 'The previous build is running. It has been left open; use the new executable in dist after closing it.'
    }
    Remove-Output $exe (Join-Path $projectDir 'build/bin')
    Get-Item -LiteralPath (Join-Path $distDir "$name.zip") | Select-Object FullName, Length
} finally {
    Remove-Output $stage $workDir
}
