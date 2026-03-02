# Launcher script for msgraph-skill binary.
# Downloads the correct pre-compiled binary on first run, then executes it.
[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Arguments
)

$ErrorActionPreference = 'Stop'

$Repo = "merill/msgraph-skill"
$BinaryName = "msgraph-skill"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$BinDir = Join-Path $ScriptDir "bin"

function Get-Platform {
    $arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
        'X64'   { 'amd64' }
        'Arm64' { 'arm64' }
        default { throw "Unsupported architecture: $_" }
    }
    return "windows_$arch"
}

function Get-LatestVersion {
    $url = "https://api.github.com/repos/$Repo/releases/latest"
    try {
        $response = Invoke-RestMethod -Uri $url -UseBasicParsing
        return $response.tag_name
    }
    catch {
        throw "Could not determine latest version: $_"
    }
}

function Get-Binary {
    param(
        [string]$Platform,
        [string]$Version
    )

    $url = "https://github.com/$Repo/releases/download/$Version/${BinaryName}_${Platform}.exe"
    $target = Join-Path $BinDir "$BinaryName.exe"

    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    }

    Write-Host "Downloading $BinaryName $Version for $Platform..." -ForegroundColor Cyan
    Invoke-WebRequest -Uri $url -OutFile $target -UseBasicParsing

    # Save version for future checks
    Set-Content -Path (Join-Path $BinDir ".version") -Value $Version

    Write-Host "Downloaded to $target" -ForegroundColor Green
}

# Main logic
$platform = Get-Platform
$binaryPath = Join-Path $BinDir "$BinaryName.exe"

# Download if binary doesn't exist
if (-not (Test-Path $binaryPath)) {
    $version = Get-LatestVersion
    Get-Binary -Platform $platform -Version $version
}

# Execute the binary with all arguments passed through
& $binaryPath @Arguments
exit $LASTEXITCODE
