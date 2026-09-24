#Requires -Version 7.0
<#
.SYNOPSIS
Builds the ado-mcp container image with Docker.

.DESCRIPTION
IMAGE overrides the image name (default: ado-mcp:latest).
#>

$ErrorActionPreference = 'Stop'

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    [Console]::Error.WriteLine('Error: docker is not installed')
    exit 1
}

Push-Location (Split-Path $PSScriptRoot -Parent)
try {
    $image = $env:IMAGE ? $env:IMAGE : 'ado-mcp:latest'
    $version = git describe --tags --always --dirty 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $version) {
        $version = 'dev'
    }

    docker build --build-arg "VERSION=$version" -t $image .
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
