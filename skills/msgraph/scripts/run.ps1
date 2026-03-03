# Launcher script for msgraph CLI.
# Executes the pre-bundled binary for the detected platform.
[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Arguments
)

$ErrorActionPreference = 'Stop'

$BinaryName = "msgraph"
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

# Main logic
$platform = Get-Platform
$binaryPath = Join-Path $BinDir "${BinaryName}_${platform}.exe"

if (-not (Test-Path $binaryPath)) {
    Write-Error "Binary not found: $binaryPath`nExpected a pre-bundled binary for platform '$platform'.`nPlease reinstall the skill or download the correct release."
    exit 1
}

& $binaryPath @Arguments
exit $LASTEXITCODE
