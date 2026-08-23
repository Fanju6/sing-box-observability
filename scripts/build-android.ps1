#requires -Version 7.0

[CmdletBinding()]
param(
    [ValidatePattern('^[0-9A-Za-z][0-9A-Za-z._+-]*$')]
    [string]$Version = '',

    [string]$OutputDirectory = ''
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$utf8NoBom = [Text.UTF8Encoding]::new($false)

function Get-RelativeFileNames {
    param([Parameter(Mandatory)][string]$Root)

    [string[]]$names = @(Get-ChildItem -LiteralPath $Root -Recurse -File -Force | ForEach-Object {
        [IO.Path]::GetRelativePath($Root, $_.FullName).Replace('\', '/')
    })
    [Array]::Sort($names, [StringComparer]::Ordinal)
    return $names
}

$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$webRoot = Join-Path $repoRoot 'src\web'
$serverRoot = Join-Path $repoRoot 'src\server'
$embedDirectory = Join-Path $serverRoot 'internal\webui\dist'
$releaseRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot 'release'))
if (Test-Path -LiteralPath $releaseRoot) {
    $releaseItem = Get-Item -LiteralPath $releaseRoot -Force
    if (($releaseItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Refusing to use reparse-point release directory: $releaseRoot"
    }
}
if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
    $OutputDirectory = Join-Path $releaseRoot 'android-arm64'
}
$outputRoot = [IO.Path]::GetFullPath($OutputDirectory)

$outputParent = [IO.Path]::GetFullPath([IO.Path]::GetDirectoryName($outputRoot))
if (-not $outputParent.Equals($releaseRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw "OutputDirectory must be a direct child of $releaseRoot"
}
if (Test-Path -LiteralPath $outputRoot) {
    $outputItem = Get-Item -LiteralPath $outputRoot -Force
    if (($outputItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Refusing to replace reparse-point output directory: $outputRoot"
    }
}

foreach ($command in @('go', 'git', 'node', 'pnpm')) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) {
        throw "Required command is unavailable: $command"
    }
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = (& git -C $repoRoot describe --tags --always --dirty 2>$null)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($Version)) {
        $Version = '0.1.0-dev'
    }
    $Version = $Version.Trim()
}
if ($Version -match '^v[0-9]') {
    $Version = $Version.Substring(1)
}
if ($Version -notmatch '^[0-9A-Za-z][0-9A-Za-z._+-]*$') {
    throw 'Version may contain only letters, digits, dot, underscore, plus, and hyphen.'
}

$commit = (& git -C $repoRoot rev-parse --verify HEAD 2>$null)
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($commit)) {
    $commit = 'unknown'
} else {
    $commit = $commit.Trim()
    $worktreeStatus = (& git -C $repoRoot status --porcelain --untracked-files=normal)
    if ($LASTEXITCODE -ne 0) { throw 'cannot inspect Git worktree state' }
    if ($worktreeStatus) { $commit += '-dirty' }
}
$buildTime = (& git -C $repoRoot show -s --format=%cI HEAD 2>$null)
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($buildTime)) {
    $buildTime = 'unknown'
} else {
    $buildTime = $buildTime.Trim()
}

Write-Host "Building frontend with locked dependencies..."
& pnpm --dir $webRoot install --frozen-lockfile
if ($LASTEXITCODE -ne 0) { throw 'pnpm install failed' }
& pnpm --dir $webRoot build
if ($LASTEXITCODE -ne 0) { throw 'frontend build failed' }

$webDist = Join-Path $webRoot 'dist'
$frontendDigestLines = @(Get-RelativeFileNames -Root $webDist | ForEach-Object {
        $relative = $_
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $webDist $relative)).Hash.ToLowerInvariant()
        "$hash  $relative"
    })
if ($frontendDigestLines.Count -eq 0) {
    throw 'frontend build produced no files'
}
$frontendDigestInput = ($frontendDigestLines -join "`n") + "`n"
$frontendHasher = [Security.Cryptography.SHA256]::Create()
try {
    $frontendDigestBytes = $frontendHasher.ComputeHash(
        [Text.Encoding]::UTF8.GetBytes($frontendDigestInput)
    )
} finally {
    $frontendHasher.Dispose()
}
$frontendDigest = -join ($frontendDigestBytes | ForEach-Object { $_.ToString('x2') })
$frontendMarkerName = "frontend-$frontendDigest.txt"
[IO.File]::WriteAllText(
    (Join-Path $webRoot "dist\$frontendMarkerName"),
    $frontendDigest + "`n",
    $utf8NoBom
)

$expectedEmbed = [IO.Path]::GetFullPath((Join-Path $repoRoot 'src\server\internal\webui\dist'))
if ([IO.Path]::GetFullPath($embedDirectory) -ne $expectedEmbed) {
    throw "Refusing to replace unexpected embed directory: $embedDirectory"
}
if (Test-Path -LiteralPath $embedDirectory) {
    $embedItem = Get-Item -LiteralPath $embedDirectory -Force
    if (($embedItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Refusing to replace reparse-point embed directory: $embedDirectory"
    }
    Remove-Item -LiteralPath $embedDirectory -Recurse -Force
}
New-Item -ItemType Directory -Path $embedDirectory -Force | Out-Null
Copy-Item -Path (Join-Path $webRoot 'dist\*') -Destination $embedDirectory -Recurse -Force

Write-Host "Testing the embedded build..."
& go -C $serverRoot test -tags webui ./...
if ($LASTEXITCODE -ne 0) { throw 'embedded Go tests failed' }

if (Test-Path -LiteralPath $outputRoot) {
    Remove-Item -LiteralPath $outputRoot -Recurse -Force
}
New-Item -ItemType Directory -Path $outputRoot -Force | Out-Null

$module = 'github.com/Fanju6/sing-box-observability/src/server/internal/buildinfo'
$linkerFlags = "-s -w -X $module.Version=$Version -X $module.Commit=$commit -X $module.BuildTime=$buildTime"
$savedGoos = $env:GOOS
$savedGoarch = $env:GOARCH
$savedCgo = $env:CGO_ENABLED
try {
    $env:GOOS = 'android'
    $env:GOARCH = 'arm64'
    $env:CGO_ENABLED = '0'
    & go -C $serverRoot build -buildvcs=false -tags webui -trimpath -ldflags $linkerFlags -o (Join-Path $outputRoot 'sing-box-observability') ./cmd/sing-box-observability
    if ($LASTEXITCODE -ne 0) { throw 'Android cross-compilation failed' }
} finally {
    if ($null -eq $savedGoos) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $savedGoos }
    if ($null -eq $savedGoarch) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue } else { $env:GOARCH = $savedGoarch }
    if ($null -eq $savedCgo) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue } else { $env:CGO_ENABLED = $savedCgo }
}

$binaryPath = Join-Path $outputRoot 'sing-box-observability'
$binaryText = [Text.Encoding]::Latin1.GetString([IO.File]::ReadAllBytes($binaryPath))
if (-not $binaryText.Contains($frontendMarkerName, [StringComparison]::Ordinal)) {
    throw "Android binary does not contain the current frontend build marker: $frontendMarkerName"
}
if ($binaryText.Contains('mockServiceWorker.js', [StringComparison]::Ordinal)) {
    throw 'Android binary unexpectedly contains mockServiceWorker.js'
}

Copy-Item (Join-Path $repoRoot 'packaging\android\config.example.yaml') (Join-Path $outputRoot 'config.yaml')
Copy-Item (Join-Path $repoRoot 'packaging\android\sing-box-observabilityctl') $outputRoot
Copy-Item (Join-Path $repoRoot 'packaging\android\service.d.sh') $outputRoot
Copy-Item (Join-Path $repoRoot 'packaging\android\README.md') $outputRoot
Copy-Item (Join-Path $repoRoot 'LICENSE') $outputRoot
Copy-Item (Join-Path $repoRoot 'NOTICE') $outputRoot
Copy-Item (Join-Path $repoRoot 'THIRD_PARTY_LICENSES.txt') $outputRoot

$manifest = @(
    "version=$Version"
    "commit=$commit"
    "buildTime=$buildTime"
    "target=android/arm64"
    "frontendDigest=$frontendDigest"
)
[IO.File]::WriteAllText(
    (Join-Path $outputRoot 'BUILD-MANIFEST.txt'),
    ($manifest -join "`n") + "`n",
    $utf8NoBom
)

$frontendLines = Get-RelativeFileNames -Root $webDist |
    ForEach-Object {
        $relative = $_
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $webDist $relative)).Hash.ToLowerInvariant()
        "$hash  $relative"
    }
[IO.File]::WriteAllText(
    (Join-Path $outputRoot 'FRONTEND-SHA256.txt'),
    ($frontendLines -join "`n") + "`n",
    $utf8NoBom
)

Write-Host "Collecting third-party license texts..."
& node (Join-Path $repoRoot 'scripts\collect-frontend-licenses.mjs') `
    --web-root $webRoot `
    --inventory (Join-Path $outputRoot 'FRONTEND-LICENSES.json') `
    --notices (Join-Path $outputRoot 'FRONTEND-LICENSES.txt')
if ($LASTEXITCODE -ne 0) { throw 'frontend license collection failed' }

& go run (Join-Path $repoRoot 'scripts\collect-go-licenses.go') `
    -server-root $serverRoot `
    -target ./cmd/sing-box-observability `
    -tags webui `
    -goos android `
    -goarch arm64 `
    -inventory (Join-Path $outputRoot 'GO-MODULES.txt') `
    -notices (Join-Path $outputRoot 'GO-LICENSES.txt')
if ($LASTEXITCODE -ne 0) { throw 'Go license collection failed' }

$checksumLines = Get-RelativeFileNames -Root $outputRoot |
    Where-Object { $_ -ne 'SHA256SUMS.txt' } |
    ForEach-Object {
        $relative = $_
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $outputRoot $relative)).Hash.ToLowerInvariant()
        "$hash  $relative"
    }
[IO.File]::WriteAllText(
    (Join-Path $outputRoot 'SHA256SUMS.txt'),
    ($checksumLines -join "`n") + "`n",
    $utf8NoBom
)

Write-Host "Android package folder: $outputRoot"
Write-Host "Version: $Version"
Write-Host "Commit: $commit"
