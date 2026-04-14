# flowcmd installer for Windows.
#
# Usage:
#   irm https://raw.githubusercontent.com/flowcmd/cli/main/install.ps1 | iex
#   $env:FLOWCMD_VERSION = "v0.1.0"; irm https://raw.githubusercontent.com/flowcmd/cli/main/install.ps1 | iex
#
# Environment:
#   FLOWCMD_VERSION      — pin a specific version (e.g. v0.1.0); default: latest release
#   FLOWCMD_INSTALL_DIR  — install destination; default: $env:LOCALAPPDATA\flowcmd\bin

$ErrorActionPreference = "Stop"

$Owner = "flowcmd"
$Repo = "cli"
$Bin = "flowcmd.exe"

# --- detect platform --------------------------------------------------------

$arch = switch -Regex ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64|x86_64" { "amd64" }
    "ARM64"        { "arm64" }
    default        { throw "unsupported arch: $env:PROCESSOR_ARCHITECTURE (supported: amd64, arm64)" }
}

# --- resolve version --------------------------------------------------------

$version = $env:FLOWCMD_VERSION
if (-not $version) {
    Write-Host "resolving latest release..."
    $release = Invoke-RestMethod "https://api.github.com/repos/$Owner/$Repo/releases/latest"
    $version = $release.tag_name
    if (-not $version) { throw "could not resolve latest release tag from GitHub API" }
}

$versionBare = $version -replace "^v", ""

$archive = "${Bin -replace '\.exe$', ''}_${versionBare}_windows_${arch}.zip"
$baseUrl = "https://github.com/$Owner/$Repo/releases/download/$version"

# --- pick install dir -------------------------------------------------------

$installDir = $env:FLOWCMD_INSTALL_DIR
if (-not $installDir) {
    $installDir = Join-Path $env:LOCALAPPDATA "flowcmd\bin"
}
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

# --- download + verify ------------------------------------------------------

$tmp = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP "flowcmd-install-$([guid]::NewGuid())")
try {
    Write-Host "downloading $archive..."
    Invoke-WebRequest -Uri "$baseUrl/$archive" -OutFile (Join-Path $tmp $archive) -UseBasicParsing

    Write-Host "verifying checksum..."
    Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt") -UseBasicParsing

    $expectedLine = Get-Content (Join-Path $tmp "checksums.txt") | Where-Object { $_ -match "\s+$([regex]::Escape($archive))$" }
    if (-not $expectedLine) { throw "no checksum listed for $archive" }
    $expected = ($expectedLine -split "\s+")[0]

    $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $archive)).Hash.ToLower()
    if ($expected.ToLower() -ne $actual) {
        throw "checksum mismatch for $archive`n  expected: $expected`n  actual:   $actual"
    }

    # --- extract + install --------------------------------------------------

    Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath $tmp -Force

    $source = Join-Path $tmp $Bin
    if (-not (Test-Path $source)) { throw "binary $Bin not found in archive" }

    $dest = Join-Path $installDir $Bin
    Copy-Item -Path $source -Destination $dest -Force

    Write-Host "`u{2713} installed $Bin to $dest"
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

# --- PATH hint --------------------------------------------------------------

$onPath = ($env:Path -split ";") -contains $installDir
if (-not $onPath) {
    Write-Host ""
    Write-Host "note: $installDir is not on your PATH. Add it for the current session:"
    Write-Host "  `$env:Path = `"$installDir;`$env:Path`""
    Write-Host "or persist it:"
    Write-Host "  [Environment]::SetEnvironmentVariable('Path', `"$installDir;`" + [Environment]::GetEnvironmentVariable('Path','User'), 'User')"
}

# --- sanity check -----------------------------------------------------------

Write-Host ""
& $dest --version
