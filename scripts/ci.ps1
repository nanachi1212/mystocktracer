<#
.SYNOPSIS
    Local validation dispatcher for mystocktracer.

.DESCRIPTION
    Auto routes changed areas to the existing Go, npm, and tooling commands.
    Full and Live remain explicit because they are broad or networked checks.
#>
param(
    [ValidateSet('Auto', 'Fast', 'Full', 'Live')]
    [string]$Mode = 'Auto',
    [string[]]$ChangedPath = @(),
    [switch]$PlanOnly
)

$ErrorActionPreference = 'Stop'
$RepoRoot = (Get-Item (Join-Path $PSScriptRoot '..')).FullName
Set-Location $RepoRoot

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)][string]$File,
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [string]$WorkingDirectory = $RepoRoot
    )

    Push-Location $WorkingDirectory
    try {
        & $File @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "$File exited with code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
}

function Get-ChangedPaths {
    if ($ChangedPath.Count -gt 0) {
        return @($ChangedPath | ForEach-Object { $_ -replace '\\', '/' })
    }

    $paths = @()
    $paths += @(git diff --name-only origin/main...HEAD 2>$null)
    $paths += @(git diff --name-only)
    $paths += @(git ls-files --others --exclude-standard)
    return @($paths | Where-Object { $_ } | Sort-Object -Unique | ForEach-Object { $_ -replace '\\', '/' })
}

function Test-PowerShellSyntax {
    $tokens = $null
    $errors = $null
    [System.Management.Automation.Language.Parser]::ParseFile(
        (Join-Path $PSScriptRoot 'ci.ps1'),
        [ref]$tokens,
        [ref]$errors
    ) | Out-Null
    if ($errors.Count -gt 0) {
        throw "PowerShell parse failed: $($errors[0].Message)"
    }
    Write-Host 'PowerShell syntax: PASS'
}

function Get-Actions {
    param([string[]]$Paths)

    $backend = $false
    $backendBuild = $false
    $frontend = $false
    $frontendBuild = $false
    $desktop = $false
    $tooling = $false
    $provenance = $false
    $full = $false

    foreach ($path in $Paths) {
        if ($path -match '^backend/') { $backend = $true }
        if ($path -match '^backend/(go\.mod|go\.sum)$') { $backendBuild = $true }
        if ($path -match '^frontend/') { $frontend = $true; $frontendBuild = $true }
        if ($path -match '^frontend/(package\.json|tsconfig|vite\.config)') { $frontendBuild = $true }
        if ($path -match '^desktop/') { $desktop = $true }
        if ($path -match '^scripts/') { $tooling = $true }
        if ($path -match '^(docs/oss-|LICENSE$|NOTICE\.md$)') { $provenance = $true }
        if ($path -match '^(package\.json|package-lock\.json)$') { $full = $true }
    }

    return [pscustomobject]@{
        Backend = $backend
        BackendBuild = $backendBuild
        Frontend = $frontend
        FrontendBuild = $frontendBuild
        Desktop = $desktop
        Tooling = $tooling
        Provenance = $provenance
        Full = $full
    }
}

function Invoke-Backend {
    param([bool]$Build, [bool]$Tests = $true)
    if ($Tests) {
        Invoke-Checked -File 'go' -Arguments @('test', './...') -WorkingDirectory (Join-Path $RepoRoot 'backend')
    }
    if ($Build) {
        Invoke-Checked -File 'go' -Arguments @('vet', './...') -WorkingDirectory (Join-Path $RepoRoot 'backend')
        Invoke-Checked -File 'go' -Arguments @('build', './...') -WorkingDirectory (Join-Path $RepoRoot 'backend')
    }
}

function Invoke-Frontend {
    param([bool]$Build)
    Invoke-Checked -File 'npm.cmd' -Arguments @('--workspace', 'frontend', 'test', '--', '--run')
    if ($Build) {
        Invoke-Checked -File 'npm.cmd' -Arguments @('run', 'build:frontend')
    }
}

function Invoke-Desktop {
    Invoke-Checked -File 'npm.cmd' -Arguments @('--workspace', 'desktop', 'test')
}

function Show-Plan {
    param([string]$SelectedMode, [string[]]$Paths, $Actions)

    Write-Host "Mode: $SelectedMode"
    Write-Host ('Changed paths: {0}' -f ($(if ($Paths.Count) { $Paths -join ', ' } else { '(none)' })))
    if ($SelectedMode -eq 'Fast') { Write-Host 'Plan: tooling tests' }
    elseif ($SelectedMode -eq 'Full') { Write-Host 'Plan: full package, backend, desktop, tooling, provenance, and frontend checks' }
    elseif ($SelectedMode -eq 'Live') { Write-Host 'Plan: official Taiwan and ToAlpha live provider tests' }
    elseif (-not ($Actions.Backend -or $Actions.Frontend -or $Actions.Desktop -or $Actions.Tooling -or $Actions.Provenance -or $Actions.Full)) {
        Write-Host 'WARN / NOT_TESTED: no affected validation target'
    }
    else {
        if ($Actions.Backend) { Write-Host 'Plan: backend tests' }
        if ($Actions.Frontend) { Write-Host 'Plan: frontend tests/build' }
        if ($Actions.Desktop) { Write-Host 'Plan: desktop tests' }
        if ($Actions.Tooling) { Write-Host 'Plan: tooling tests and PowerShell syntax' }
        if ($Actions.Provenance) { Write-Host 'Plan: provenance audit' }
        if ($Actions.Full) { Write-Host 'Plan: full package checks' }
    }
}

$paths = @(Get-ChangedPaths)
$actions = Get-Actions -Paths $paths
Show-Plan -SelectedMode $Mode -Paths $paths -Actions $actions
if ($PlanOnly) { exit 0 }

switch ($Mode) {
    'Fast' {
        Invoke-Checked -File 'npm.cmd' -Arguments @('run', 'test:tooling')
        Test-PowerShellSyntax
    }
    'Full' {
        Invoke-Checked -File 'npm.cmd' -Arguments @('test')
        Invoke-Backend -Build $true -Tests $false
        Invoke-Desktop
        Invoke-Checked -File 'npm.cmd' -Arguments @('run', 'test:tooling')
        Invoke-Checked -File 'npm.cmd' -Arguments @('run', 'audit:provenance', '--', '--summary')
        Invoke-Checked -File 'npm.cmd' -Arguments @('run', 'build:frontend')
        Test-PowerShellSyntax
    }
    'Live' {
        $env:EASY_STOCK_TW_LIVE_TEST = '1'
        try {
            Invoke-Checked -File 'go' -Arguments @('test', './internal/providers/taiwan', '-run', 'Live', '-v') -WorkingDirectory (Join-Path $RepoRoot 'backend')
        } finally {
            Remove-Item Env:EASY_STOCK_TW_LIVE_TEST -ErrorAction SilentlyContinue
        }
        $env:A_STOCK_LIVE_TEST = '1'
        try {
            Invoke-Checked -File 'go' -Arguments @('test', './internal/providers/toalpha', '-run', 'Live', '-v') -WorkingDirectory (Join-Path $RepoRoot 'backend')
        } finally {
            Remove-Item Env:A_STOCK_LIVE_TEST -ErrorAction SilentlyContinue
        }
    }
    'Auto' {
        if ($actions.Full) {
            Invoke-Checked -File 'npm.cmd' -Arguments @('test')
            Invoke-Backend -Build $true -Tests $false
            Invoke-Desktop
            Invoke-Checked -File 'npm.cmd' -Arguments @('run', 'build:frontend')
        } else {
            if ($actions.Backend) { Invoke-Backend -Build $actions.BackendBuild }
            if ($actions.Frontend) { Invoke-Frontend -Build $actions.FrontendBuild }
            if ($actions.Desktop) { Invoke-Desktop }
            if ($actions.Tooling) { Invoke-Checked -File 'npm.cmd' -Arguments @('run', 'test:tooling'); Test-PowerShellSyntax }
            if ($actions.Provenance) { Invoke-Checked -File 'npm.cmd' -Arguments @('run', 'audit:provenance', '--', '--summary') }
            if (-not ($actions.Backend -or $actions.Frontend -or $actions.Desktop -or $actions.Tooling -or $actions.Provenance)) {
                Write-Host 'WARN / NOT_TESTED: unrelated paths only'
            }
        }
    }
}

Write-Host 'Local CI: PASS'
