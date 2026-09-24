#Requires -Version 7.0
<#
.SYNOPSIS
Builds the ado-mcp and create-pat binaries into bin/.

.DESCRIPTION
GOOS and GOARCH select another target platform. Binaries built for Windows get the .exe
extension.
#>

$ErrorActionPreference = 'Stop'

Push-Location (Split-Path $PSScriptRoot -Parent)
$savedCgo = $env:CGO_ENABLED
try {
    $version = git describe --tags --always --dirty 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $version) {
        $version = 'dev'
    }
    $extension = (go env GOOS) -eq 'windows' ? '.exe' : ''

    $env:CGO_ENABLED = '0'
    foreach ($command in 'ado-mcp', 'create-pat') {
        $output = "bin/$command$extension"
        go build -trimpath -ldflags "-s -w -X main.version=$version" -o $output "./cmd/$command"
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
        Write-Output "$output $version"
    }
}
finally {
    $env:CGO_ENABLED = $savedCgo
    Pop-Location
}
