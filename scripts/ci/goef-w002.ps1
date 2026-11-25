#Requires -Version 7.0
[CmdletBinding()]
param(
    [switch] $AcceptSqlServerEula,
    [switch] $Compatibility
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false

function Invoke-Check {
    param(
        [string] $Application,
        [string[]] $Arguments
    )

    $command = "$(Split-Path -Leaf $Application) $($Arguments -join ' ')"
    Write-Host "RUN $command"
    & $Application @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$command exited with code $LASTEXITCODE."
    }
    Write-Host "PASS $command"
}

function Confirm-DriverResults {
    param(
        [string[]] $Lines,
        [int] $ExitCode,
        [System.Collections.IDictionary] $ExpectedProfiles
    )

    $scenarioIds = 'LAB-DRIVER-001-S01', 'LAB-DRIVER-001-S02', 'LAB-DRIVER-001-S03'
    $seenProviders = [System.Collections.Generic.HashSet[string]]::new()
    $failures = [System.Collections.Generic.List[string]]::new()
    if ($ExitCode -ne 0) {
        $failures.Add("Driver runner exited with code $ExitCode.")
    }

    $lineNumber = 0
    foreach ($line in $Lines) {
        $lineNumber++
        try {
            $row = ConvertFrom-Json -InputObject $line -AsHashtable -NoEnumerate
        }
        catch {
            $failures.Add("Driver report line $lineNumber is invalid JSON.")
            continue
        }

        if ($row -isnot [System.Collections.IDictionary] -or
            $row['provider'] -isnot [string] -or $row['provider'] -cnotin $ExpectedProfiles.Keys) {
            $failures.Add("Driver report line $lineNumber has no recognized provider.")
            continue
        }
        $provider = $row['provider']
        if (-not $seenProviders.Add($provider)) {
            $failures.Add("Duplicate driver result for $provider.")
            continue
        }
        $expectedProfile = $ExpectedProfiles[$provider]
        if ($row['profile'] -isnot [string] -or $row['profile'] -cne $expectedProfile) {
            $failures.Add("The $provider driver result must identify profile $expectedProfile.")
            continue
        }
        if ($row['status'] -isnot [string] -or $row['status'] -cnotin ('PASS', 'FAIL', 'BLOCKED')) {
            $failures.Add("Invalid driver status for $provider.")
            continue
        }

        # The driver runner emits redacted evidence on stdout.
        Write-Output $line
        if ($row['status'] -cne 'PASS') {
            $failures.Add("$provider driver result is $($row['status']).")
            continue
        }
        if ($row['scenarios'] -isnot [array]) {
            $failures.Add("Missing driver scenario list for $provider.")
            continue
        }

        $seenScenarios = [System.Collections.Generic.HashSet[string]]::new()
        foreach ($result in $row['scenarios']) {
            if ($result -isnot [System.Collections.IDictionary] -or
                $result['provider'] -isnot [string] -or $result['provider'] -cne $provider -or
                $result['scenario'] -isnot [string] -or $result['scenario'] -cnotin $scenarioIds) {
                $failures.Add("Unexpected driver scenario for $provider.")
                continue
            }
            $scenario = $result['scenario']
            if (-not $seenScenarios.Add($scenario)) {
                $failures.Add("Duplicate driver scenario $scenario for $provider.")
            }
            if ($result['status'] -isnot [string] -or $result['status'] -cne 'PASS') {
                $failures.Add("Driver scenario $scenario for $provider did not pass.")
            }
        }
        foreach ($scenario in $scenarioIds) {
            if (-not $seenScenarios.Contains($scenario)) {
                $failures.Add("Missing driver scenario $scenario for $provider.")
            }
        }
    }
    foreach ($provider in $ExpectedProfiles.Keys) {
        if (-not $seenProviders.Contains($provider)) {
            $failures.Add("Missing driver result for $provider.")
        }
    }
    if ($failures.Count -gt 0) {
        throw ($failures -join [Environment]::NewLine)
    }
}

function Invoke-DriverMatrix {
    param(
        [string] $Python,
        [string[]] $Selection,
        [System.Collections.IDictionary] $ExpectedProfiles
    )

    $driverArguments = @('-B', 'test/conformance/lab/driver_run.py') + $Selection
    Write-Host "RUN $(Split-Path -Leaf $Python) $($driverArguments -join ' ')"
    $driverOutput = @(& $Python @driverArguments)
    Confirm-DriverResults -Lines $driverOutput -ExitCode $LASTEXITCODE -ExpectedProfiles $ExpectedProfiles
}

$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
Push-Location -LiteralPath $repoRoot
$savedToolchain = $env:GOTOOLCHAIN
$savedWorkspace = $env:GOWORK
$savedProvider = $env:GOEF_DRIVERLAB_PROVIDER
try {
    $env:GOTOOLCHAIN = 'local'
    $env:GOWORK = 'off'
    $env:GOEF_DRIVERLAB_PROVIDER = $null

    $go = (Get-Command go -CommandType Application | Select-Object -First 1).Path
    $python = (Get-Command python -CommandType Application | Select-Object -First 1).Path
    $version = & $go version
    if ($LASTEXITCODE -ne 0 -or $version -isnot [string] -or $version -cnotmatch '^go version go1\.27\.4 \S+$') {
        throw 'The local Go toolchain must be exactly Go 1.27.4.'
    }
    Write-Host "PASS $version"

    Invoke-Check $python @('-B', 'test/bootstrap/verify.py')
    Invoke-Check $python @('-B', '-m', 'unittest', 'discover', '-s', 'test/bootstrap', '-p', 'test_verify.py')
    Invoke-Check $python @('-B', '-m', 'unittest', 'discover', '-s', 'test/conformance/lab', '-p', 'test*.py')

    Write-Host 'Go module: repository root'
    Invoke-Check $go @('mod', 'tidy', '-diff')
    Invoke-Check $go @('build', '-mod=readonly', './...')
    Invoke-Check $go @('vet', '-mod=readonly', './...')
    Invoke-Check $go @('test', '-mod=readonly', '-shuffle=on', '-count=1', './...')

    Write-Host 'Go module: test/conformance/driverlab'
    Push-Location -LiteralPath 'test/conformance/driverlab'
    try {
        Invoke-Check $go @('mod', 'verify')
        $driverPins = [ordered] @{
            'github.com/jackc/pgx/v5' = 'v5.13.0'
            'modernc.org/sqlite' = 'v1.63.0'
            'github.com/microsoft/go-mssqldb' = 'v1.11.0'
            'github.com/go-sql-driver/mysql' = 'v1.10.0'
        }
        $format = '{{.Path}} {{.Version}}{{if .Replace}} (replaced){{end}}{{if .Indirect}} (indirect){{end}}'
        $pinArguments = @('list', '-mod=readonly', '-m', '-f', $format) + @($driverPins.Keys)
        $selectedPins = @(Invoke-Check $go $pinArguments)
        $expectedPins = @($driverPins.GetEnumerator() | ForEach-Object { "$($_.Key) $($_.Value)" })
        if ($selectedPins.Count -ne $expectedPins.Count -or
            (Compare-Object $expectedPins $selectedPins -CaseSensitive)) {
            throw "The driver module must select these exact direct dependencies: $($expectedPins -join ', ')."
        }
        foreach ($pin in $selectedPins) {
            Write-Host "PASS driver pin $pin"
        }
        Invoke-Check $go @('mod', 'tidy', '-diff')
        Invoke-Check $go @('build', '-mod=readonly', './...')
        Invoke-Check $go @('vet', '-mod=readonly', './...')
        Invoke-Check $go @('test', '-mod=readonly', '-shuffle=on', '-count=1', './...')
    }
    finally {
        Pop-Location
    }

    $selection = @('all')
    if ($AcceptSqlServerEula) {
        $selection += '--accept-sqlserver-eula'
    }
    $baselineProfiles = [ordered] @{
        'postgres' = 'postgres-18'
        'sqlite' = 'sqlite-3.54'
        'sqlserver' = 'sqlserver-2022'
        'mysql' = 'mysql-8.4'
    }
    Invoke-DriverMatrix -Python $python -Selection $selection -ExpectedProfiles $baselineProfiles
    Write-Host 'PASS W002 local checks and all four driver suites'
    if ($Compatibility) {
        $profiles = [ordered] @{
            'postgres' = 'postgres-16'
            'mysql' = 'mysql-8.0'
        }
        $compatibilityFailures = [System.Collections.Generic.List[string]]::new()
        foreach ($entry in $profiles.GetEnumerator()) {
            $provider = $entry.Key
            $profileName = $entry.Value
            $selection = $provider, '--profile', $profileName
            try {
                Invoke-DriverMatrix -Python $python -Selection $selection -ExpectedProfiles @{ $provider = $profileName }
                Write-Host "PASS W002 $profileName compatibility suite"
            }
            catch {
                $compatibilityFailures.Add("$profileName failed. $($_.Exception.Message)")
            }
        }
        if ($compatibilityFailures.Count -gt 0) {
            throw ($compatibilityFailures -join [Environment]::NewLine)
        }
    }
}
finally {
    $env:GOTOOLCHAIN = $savedToolchain
    $env:GOWORK = $savedWorkspace
    $env:GOEF_DRIVERLAB_PROVIDER = $savedProvider
    Pop-Location
}
