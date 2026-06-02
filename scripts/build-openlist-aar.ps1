<#
.SYNOPSIS
    Build the OpenList Android AAR (gomobile bind product) from the Hi-Sillot
    OpenList fork on Windows (CI matrix uses this for the windows-latest leg).

.DESCRIPTION
    Drop-in PowerShell 7+ equivalent of scripts/build-openlist-aar.sh. Produces
    <output>/openlist.aar (gomobile bind product) for ComboLite to consume via
    app/encv-mobile/plugin-openlist/libs/openlist.aar.

.PARAMETER Output
    Output directory for openlist.aar (required).

.PARAMETER Fork
    Hi-Sillot fork URL. Default: https://github.com/Hi-Sillot/OpenList

.PARAMETER Branch
    Git branch / tag. Default: main

.PARAMETER Ndk
    Android NDK install path. Default: $env:ANDROID_HOME\ndk\26.3.11579264

.PARAMETER EncvGoRoot
    Local encv-go checkout (encv-go replace target). Default: C:\workspace

.EXAMPLE
    pwsh -File scripts/build-openlist-aar.ps1 `
        -Output  C:\workspace\app\encv-mobile\plugin-openlist\libs `
        -EncvGoRoot C:\workspace

.NOTES
    Required environment:
      - Go 1.25.x          (matches Hi-Sillot fork go.mod)
      - NDK r25c+          (r26b / 26.3.11579264 recommended)
      - Java 17            (Temurin / OpenJDK)
      - cmake, git, curl, tar (tar via bsdtar / 7-Zip)
#>
# TODO: keep NDK version in sync with .github/workflows/build-mpv-lib.yml.
# TODO: Hi-Sillot fork must already contain `openlistlib/` (see
#       .trae/specs/integrate-openlist-as-combolite-plugin/spec.md §一).

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Output,

    [string]$Fork = 'https://github.com/Hi-Sillot/OpenList',

    [string]$Branch = 'main',

    [string]$Ndk = "$env:ANDROID_HOME\ndk\26.3.11579264",

    [string]$EncvGoRoot = 'C:\workspace'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Status {
    param([string]$Message)
    Write-Host "[openlist-aar] $Message" -ForegroundColor Cyan
}

function Write-Fatal {
    param([string]$Message)
    Write-Host "[openlist-aar] $Message" -ForegroundColor Red
    exit 1
}

function Resolve-Tool {
    param([string]$Command)
    $found = (Get-Command $Command -ErrorAction SilentlyContinue)
    if (-not $found) {
        Write-Fatal "Required tool '$Command' not found in PATH"
    }
    return $found.Source
}

# -----------------------------------------------------------------------------
# Validate required tools (fail fast, identical semantics to the bash script).
# -----------------------------------------------------------------------------
Write-Status "== Environment check =="
$null = Resolve-Tool 'go'
$null = Resolve-Tool 'java'
$null = Resolve-Tool 'git'
$null = Resolve-Tool 'curl'
$null = Resolve-Tool 'tar'
$null = Resolve-Tool 'cmake'

# sha256sum exists on GitHub Actions windows-latest; fall back to .NET if absent.
$hasSha256 = [bool] (Get-Command 'sha256sum' -ErrorAction SilentlyContinue)

if (-not (Test-Path -LiteralPath $EncvGoRoot)) {
    Write-Fatal "encv-go root not found: $EncvGoRoot"
}
$EncvGoRoot = (Resolve-Path -LiteralPath $EncvGoRoot).ProviderPath.TrimEnd('\', '/')

# Resolve NDK: walk a few common locations if the explicit value is missing.
if (-not (Test-Path -LiteralPath $Ndk)) {
    $candidates = @(
        (Join-Path $env:ANDROID_HOME 'ndk\26.3.11579264'),
        (Join-Path $env:ANDROID_HOME 'ndk\25.2.9519653'),
        'C:\Android\ndk\26.3.11579264',
        'C:\Android\ndk\25.2.9519653'
    )
    foreach ($cand in $candidates) {
        if ($cand -and (Test-Path -LiteralPath $cand)) {
            $Ndk = $cand
            break
        }
    }
}
if (-not (Test-Path -LiteralPath $Ndk)) {
    Write-Fatal "NDK not found at: $Ndk"
}
$Ndk = (Resolve-Path -LiteralPath $Ndk).ProviderPath
$ndkBuild = Join-Path $Ndk 'ndk-build.cmd'
if (-not (Test-Path -LiteralPath $ndkBuild)) {
    Write-Fatal "ndk-build.cmd not found under $Ndk"
}

if (-not (Test-Path -LiteralPath $Output)) {
    New-Item -ItemType Directory -Path $Output -Force | Out-Null
}
$Output = (Resolve-Path -LiteralPath $Output).ProviderPath

Write-Status "== Toolchain =="
Write-Status "  go         : $(go version)"
Write-Status "  java       : $((java -version) 2>&1 | Select-Object -First 1)"
Write-Status "  NDK        : $Ndk"
Write-Status "  encv-go    : $EncvGoRoot"
Write-Status "  fork       : $Fork@$Branch"
Write-Status "  output dir : $Output"

# -----------------------------------------------------------------------------
# Workspace + clone
# -----------------------------------------------------------------------------
$tmpRoot = $env:TEMP
if ([string]::IsNullOrEmpty($tmpRoot)) { $tmpRoot = 'C:\Temp' }
if (-not (Test-Path -LiteralPath $tmpRoot)) {
    New-Item -ItemType Directory -Path $tmpRoot -Force | Out-Null
}
$workDir  = Join-Path $tmpRoot 'openlist-aar-build'
$srcDir   = Join-Path $workDir 'openlist'
if (Test-Path -LiteralPath $srcDir) {
    Remove-Item -LiteralPath $srcDir -Recurse -Force
}
if (-not (Test-Path -LiteralPath $workDir)) {
    New-Item -ItemType Directory -Path $workDir -Force | Out-Null
}

Write-Status "== Clone Hi-Sillot fork (--depth 1) =="
git clone --depth 1 --branch $Branch $Fork $srcDir

$goMod = Join-Path $srcDir 'go.mod'
if (-not (Test-Path -LiteralPath $goMod)) {
    Write-Fatal "go.mod not found in $srcDir"
}

# -----------------------------------------------------------------------------
# Patch the encv-go replace directive (the fork ships it as ../../../
# which is meaningless once cloned into a temp dir).
# -----------------------------------------------------------------------------
Write-Status "== Patch go.mod replace directive =="
$replaced = $false
$lines = Get-Content -LiteralPath $goMod
$pattern = '^[ \t]*replace[ \t]+github\.com/Soltus/encv-go[ \t]+=>[ \t]+([^\s]+)'
for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -match $pattern) {
        $lines[$i] = "replace github.com/Soltus/encv-go => $EncvGoRoot"
        $replaced = $true
        break
    }
}
if (-not $replaced) {
    $lines += "replace github.com/Soltus/encv-go => $EncvGoRoot"
}
Set-Content -LiteralPath $goMod -Value $lines -Encoding UTF8

# -----------------------------------------------------------------------------
# Frontend dist — required at runtime (OpenList embeds public/dist).
# -----------------------------------------------------------------------------
Write-Status "== Fetch OpenList-Frontend dist =="
$distDir = Join-Path $srcDir 'public\dist'
New-Item -ItemType Directory -Path $distDir -Force | Out-Null

$feApi = 'https://api.github.com/repos/OpenListTeam/OpenList-Frontend/releases/latest'
$releaseInfo = (Invoke-RestMethod -Uri $feApi -TimeoutSec 15 -Headers @{ Accept = 'application/vnd.github.v3+json' })
$asset = $releaseInfo.assets |
    Where-Object { $_.browser_download_url -match 'openlist-frontend-dist.*\.tar\.gz$' } |
    Where-Object { $_.browser_download_url -notmatch 'openlist-frontend-dist-lite' } |
    Select-Object -First 1

if (-not $asset) {
    Write-Fatal "could not resolve frontend tarball URL from $feApi"
}
$dlUrl = $asset.browser_download_url
Write-Status "  frontend: $dlUrl"

$tmpTar = Join-Path $workDir 'openlist-frontend-dist.tar.gz'
Invoke-WebRequest -Uri $dlUrl -OutFile $tmpTar -TimeoutSec 60
# Windows 10+ ships bsdtar as `tar`; it understands -xzf.
tar -xzf $tmpTar -C $distDir --strip-components 1
Remove-Item -LiteralPath $tmpTar -Force

if (-not (Test-Path -LiteralPath (Join-Path $distDir 'index.html'))) {
    Write-Fatal "frontend dist extraction failed (no index.html)"
}

# -----------------------------------------------------------------------------
# NDK env + gomobile toolchain
# -----------------------------------------------------------------------------
Write-Status "== Set up NDK env =="
if (-not (Test-Path env:ANDROID_HOME) -or [string]::IsNullOrEmpty($env:ANDROID_HOME)) {
    $env:ANDROID_HOME = Split-Path -Path (Split-Path -Path $Ndk -Parent) -Parent
}
$env:ANDROID_NDK_HOME = $Ndk
Write-Status "  ANDROID_HOME=$env:ANDROID_HOME"
Write-Status "  ANDROID_NDK_HOME=$env:ANDROID_NDK_HOME"

Write-Status "== Install / update gomobile =="
$gopathBin = Join-Path (go env GOPATH) 'bin'
if (-not (Test-Path -LiteralPath $gopathBin)) {
    New-Item -ItemType Directory -Path $gopathBin -Force | Out-Null
}
$env:PATH = "$gopathBin;$env:PATH"
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init -ndk $Ndk

# -----------------------------------------------------------------------------
# Resolve the gobindable package directory (must exist in the fork).
# -----------------------------------------------------------------------------
Set-Location -LiteralPath $srcDir
$bindPkg = $null
$direct = Join-Path $srcDir 'openlistlib'
$cmdSub = Join-Path $srcDir 'cmd\openlistlib'
if ((Test-Path -LiteralPath $direct) -and (Get-ChildItem -LiteralPath $direct -Filter '*.go' -ErrorAction SilentlyContinue | Select-Object -First 1)) {
    $bindPkg = './openlistlib'
} elseif ((Test-Path -LiteralPath $cmdSub) -and (Get-ChildItem -LiteralPath $cmdSub -Filter '*.go' -ErrorAction SilentlyContinue | Select-Object -First 1)) {
    $bindPkg = './cmd/openlistlib'
} else {
    Write-Fatal "Hi-Sillot fork is missing openlistlib/ (see spec §一) and no fallback exists"
}
Write-Status "== gomobile bind (bind pkg: $bindPkg) =="

$builtAt  = Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz'
$gitHash  = (git -C $srcDir rev-parse --short HEAD)
$ldFlags  = '-s -w'
$ldFlags += " -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.Version=$Branch'"
$ldFlags += " -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.WebVersion=rolling'"
$ldFlags += " -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.BuiltAt=$builtAt'"
$ldFlags += " -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.GitAuthor=The OpenList Projects Contributors <noreply@openlist.team>'"
$ldFlags += " -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.GitCommit=$gitHash'"

$outputAar = Join-Path $Output 'openlist.aar'
gomobile bind `
    -ldflags $ldFlags `
    -v `
    -androidapi 19 `
    -target 'android/arm64' `
    -o $outputAar `
    $bindPkg

if (-not (Test-Path -LiteralPath $outputAar) -or ((Get-Item -LiteralPath $outputAar).Length -le 0)) {
    Write-Fatal "openlist.aar was not produced"
}

# -----------------------------------------------------------------------------
# Checksum
# -----------------------------------------------------------------------------
Write-Status "== Checksum =="
$shaFile = Join-Path $Output 'openlist.aar.sha256'
if ($hasSha256) {
    Push-Location -LiteralPath $Output
    sha256sum openlist.aar | Out-File -FilePath $shaFile -Encoding ascii
    Pop-Location
} else {
    $h = (Get-FileHash -LiteralPath $outputAar -Algorithm SHA256).Hash.ToLower()
    "$h  openlist.aar" | Out-File -FilePath $shaFile -Encoding ascii
}
Get-Content -LiteralPath $shaFile

Write-Status "== Done =="
Write-Status "  AAR  : $outputAar"
$sizeBytes = (Get-Item -LiteralPath $outputAar).Length
$sizeMb    = '{0:N2} MB' -f ($sizeBytes / 1MB)
Write-Status "  SIZE : $sizeMb"
