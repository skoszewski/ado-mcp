#Requires -Version 7.0
<#
.SYNOPSIS
Runs the ado-mcp container image with Docker.

.DESCRIPTION
Arguments are passed to ado-mcp. Credentials come from .env in the repository root when it
exists, and from AZURE_DEVOPS_PAT, AZURE_TENANT_ID, AZURE_CLIENT_ID and AZURE_CLIENT_SECRET when
they are set. IMAGE overrides the image name (default: ado-mcp:latest) and PORT the host port
published on 127.0.0.1 (default: 8888).
#>

$ErrorActionPreference = 'Stop'

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    [Console]::Error.WriteLine('Error: docker is not installed')
    exit 1
}

$root = Split-Path $PSScriptRoot -Parent
$image = $env:IMAGE ? $env:IMAGE : 'ado-mcp:latest'
$port = $env:PORT ? $env:PORT : '8888'

$runArgs = @('--rm', '-i', '-p', "127.0.0.1:${port}:8888")
$envFile = Join-Path $root '.env'
if (Test-Path $envFile) {
    $runArgs += '--env-file', $envFile
}
foreach ($name in 'AZURE_DEVOPS_PAT', 'AZURE_TENANT_ID', 'AZURE_CLIENT_ID', 'AZURE_CLIENT_SECRET') {
    if ([Environment]::GetEnvironmentVariable($name)) {
        $runArgs += '-e', $name
    }
}

docker run @runArgs $image @args
exit $LASTEXITCODE
