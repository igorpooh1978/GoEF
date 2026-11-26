#Requires -Version 7.0
[CmdletBinding()]
param(
    [switch] $AcceptSqlServerEula,
    [switch] $AcceptLegacySqlServerEula,
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
        [System.Collections.IDictionary] $ExpectedProfiles,
        [System.Collections.IDictionary] $ExpectedFacts = @{}
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
                continue
            }
            if ($ExpectedFacts.Contains($provider)) {
                $facts = $result['facts']
                foreach ($expected in $ExpectedFacts[$provider].GetEnumerator()) {
                    if ($facts -isnot [System.Collections.IDictionary] -or
                        $facts[$expected.Key] -isnot [string] -or $facts[$expected.Key] -cne $expected.Value) {
                        $failures.Add("Driver scenario $scenario for $expectedProfile must report $($expected.Key)=$($expected.Value).")
                    }
                }
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
        [System.Collections.IDictionary] $ExpectedProfiles,
        [System.Collections.IDictionary] $ExpectedFacts = @{}
    )

    $driverArguments = @('-B', 'test/conformance/lab/driver_run.py') + $Selection
    Write-Host "RUN $(Split-Path -Leaf $Python) $($driverArguments -join ' ')"
    $driverOutput = @(& $Python @driverArguments)
    Confirm-DriverResults -Lines $driverOutput -ExitCode $LASTEXITCODE -ExpectedProfiles $ExpectedProfiles -ExpectedFacts $ExpectedFacts
}

function Invoke-SqliteCompatibility {
    param([string] $Python)

    $script = 'test/conformance/lab/sqlite_compat.py'
    Write-Host "RUN $(Split-Path -Leaf $Python) -B $script"
    $lines = @(& $Python -B $script)
    $exitCode = $LASTEXITCODE
    if ($lines.Count -ne 1) {
        throw "SQLite compatibility must emit one JSON report. Exit code $exitCode."
    }
    try {
        $report = ConvertFrom-Json -InputObject $lines[0] -AsHashtable -NoEnumerate
    }
    catch {
        throw "SQLite compatibility emitted invalid JSON. Exit code $exitCode."
    }
    if ($report -isnot [System.Collections.IDictionary] -or
        $report['status'] -isnot [string] -or $report['status'] -cnotin ('PASS', 'FAIL', 'BLOCKED')) {
        throw "SQLite compatibility emitted an invalid status. Exit code $exitCode."
    }
    # The SQLite runner emits redacted evidence on stdout.
    Write-Output $lines[0]
    if ($exitCode -ne 0 -or $report['status'] -cne 'PASS') {
        throw "SQLite compatibility reported $($report['status']) and exited with code $exitCode."
    }

    $profiles = [ordered] @{
        'sqlite-3.51' = @{
            engine_version = '3.51.3'
            source_id = '2026-03-13 10:38:09 737ae4a34738ffa0c3ff7f9bb18df914dd1cad163f28fd6b6e114a344fe6d618'
            driver_version = 'v1.47.0'
        }
        'sqlite-3.54' = @{
            engine_version = '3.54.1'
            source_id = '2026-10-16 14:45:20 555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76'
            driver_version = 'v1.63.0'
        }
    }
    $reportedProfiles = $report['profiles']
    if ($reportedProfiles -isnot [System.Collections.IDictionary] -or $reportedProfiles.Count -ne $profiles.Count) {
        throw 'SQLite compatibility must report both exact linked runtimes.'
    }
    foreach ($profileName in $profiles.Keys) {
        $observed = $reportedProfiles[$profileName]
        foreach ($pin in $profiles[$profileName].GetEnumerator()) {
            if ($observed -isnot [System.Collections.IDictionary] -or
                $observed[$pin.Key] -isnot [string] -or $observed[$pin.Key] -cne $pin.Value) {
                throw "SQLite compatibility for $profileName must report $($pin.Key)=$($pin.Value)."
            }
        }
    }

    $nativeCodes = @{
        'LAB-DRIVER-001-S01' = $null
        'LAB-DRIVER-001-S02' = '2067'
        'LAB-DRIVER-001-S03' = '787'
    }
    $driverScenarios = $report['driver_scenarios']
    if ($driverScenarios -isnot [array] -or $driverScenarios.Count -ne ($profiles.Count * $nativeCodes.Count)) {
        throw 'SQLite compatibility must report three driver scenarios for each linked runtime.'
    }
    $seenDriverScenarios = [System.Collections.Generic.HashSet[string]]::new()
    foreach ($result in $driverScenarios) {
        if ($result -isnot [System.Collections.IDictionary] -or
            $result['provider'] -isnot [string] -or $result['provider'] -cne 'sqlite' -or
            $result['profile'] -isnot [string] -or $result['profile'] -cnotin $profiles.Keys -or
            $result['scenario'] -isnot [string] -or $result['scenario'] -cnotin $nativeCodes.Keys -or
            $result['status'] -isnot [string] -or $result['status'] -cne 'PASS') {
            throw 'SQLite compatibility emitted an unexpected or failed driver scenario.'
        }
        $profileName = $result['profile']
        $scenario = $result['scenario']
        if (-not $seenDriverScenarios.Add("$profileName/$scenario")) {
            throw "SQLite compatibility has a duplicate driver scenario for $profileName/$scenario."
        }
        $facts = $result['facts']
        $nativeCode = $nativeCodes[$scenario]
        if ($facts -isnot [System.Collections.IDictionary] -or
            $facts['engine_version'] -isnot [string] -or $facts['engine_version'] -cne $profiles[$profileName].engine_version -or
            $facts['source_id'] -isnot [string] -or $facts['source_id'] -cne $profiles[$profileName].source_id -or
            ($null -ne $nativeCode -and ($facts['native_code'] -isnot [string] -or $facts['native_code'] -cne $nativeCode)) -or
            ($null -eq $nativeCode -and $facts.Contains('native_code'))) {
            throw "SQLite compatibility reported incorrect engine identity or native code for $profileName/$scenario."
        }
    }

    $directions = @{
        'LAB-SQLITE-FILE-001-S01' = @('3.51.3', '3.54.1', '3.51.3')
        'LAB-SQLITE-FILE-001-S02' = @('3.54.1', '3.51.3', '3.54.1')
    }
    $modes = 'delete', 'wal'
    $checks = $report['checks']
    if ($checks -isnot [array] -or $checks.Count -ne ($directions.Count * $modes.Count)) {
        throw 'SQLite compatibility must report both file directions in DELETE and WAL modes.'
    }
    $seenChecks = [System.Collections.Generic.HashSet[string]]::new()
    foreach ($check in $checks) {
        if ($check -isnot [System.Collections.IDictionary] -or
            $check['scenario'] -isnot [string] -or $check['scenario'] -cnotin $directions.Keys -or
            $check['journal_mode'] -isnot [string] -or $check['journal_mode'] -cnotin $modes -or
            $check['status'] -isnot [string] -or $check['status'] -cne 'PASS') {
            throw 'SQLite compatibility emitted an unexpected or failed file check.'
        }
        $engines = $check['engines']
        $expectedEngines = $directions[$check['scenario']]
        if ($engines -isnot [array] -or $engines.Count -ne $expectedEngines.Count) {
            throw 'SQLite compatibility must report the engine for each file stage.'
        }
        for ($index = 0; $index -lt $engines.Count; $index++) {
            if ($engines[$index] -isnot [string] -or $engines[$index] -cne $expectedEngines[$index]) {
                throw 'SQLite compatibility reported an incorrect file engine sequence.'
            }
        }
        if (-not $seenChecks.Add("$($check['scenario'])/$($check['journal_mode'])")) {
            throw 'SQLite compatibility emitted a duplicate file direction and journal mode.'
        }
    }
    Write-Host 'PASS W002 SQLite linked-runtime and file compatibility suite'
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

    $currentSqlServerConsent = @{
        Accepted = $AcceptSqlServerEula
        RunnerFlag = '--accept-sqlserver-eula'
        ScriptFlag = '-AcceptSqlServerEula'
    }
    $selection = @('all')
    if ($currentSqlServerConsent.Accepted) {
        $selection += $currentSqlServerConsent.RunnerFlag
    }
    $baselineProfiles = [ordered] @{
        'postgres' = 'postgres-18'
        'sqlite' = 'sqlite-3.54'
        'sqlserver' = 'sqlserver-2022'
        'mysql' = 'mysql-8.4'
    }
    $baselineFacts = @{ sqlserver = @{ version = '16.0.4325.4'; compatibility_level = '160'; generate_series = 'available' } }
    $matrixFailures = [System.Collections.Generic.List[string]]::new()
    try {
        Invoke-DriverMatrix -Python $python -Selection $selection -ExpectedProfiles $baselineProfiles -ExpectedFacts $baselineFacts
        Write-Host 'PASS W002 local checks and all four driver suites'
    }
    catch {
        $matrixFailures.Add($_.Exception.Message)
    }
    if ($Compatibility) {
        $legacySqlServerConsent = @{
            Accepted = $AcceptLegacySqlServerEula
            RunnerFlag = '--accept-legacy-sqlserver-eula'
            ScriptFlag = '-AcceptLegacySqlServerEula'
        }
        $profiles = [ordered] @{
            'postgres-16' = @{ Provider = 'postgres'; Facts = @{ engine_version = '16.16'; server_version_num = '160016' } }
            'postgres-17' = @{ Provider = 'postgres'; Facts = @{ engine_version = '17.12'; server_version_num = '170012' } }
            'postgres-19' = @{ Provider = 'postgres'; Facts = @{ engine_version = '19.0'; server_version_num = '190000' } }
            'mysql-8.0' = @{ Provider = 'mysql'; Facts = @{ server_version = '8.0.46' } }
            'sqlserver-2017' = @{
                Provider = 'sqlserver'
                Consent = $legacySqlServerConsent
                Facts = @{ version = '14.0.3570.2'; compatibility_level = '140' }
            }
            'sqlserver-2019' = @{
                Provider = 'sqlserver'
                Consent = $legacySqlServerConsent
                Facts = @{ version = '15.0.4510.2'; compatibility_level = '150' }
            }
            'sqlserver-2022-140' = @{
                Provider = 'sqlserver'
                Consent = $currentSqlServerConsent
                Facts = @{ version = '16.0.4325.4'; compatibility_level = '140'; generate_series = 'unavailable' }
            }
        }
        foreach ($entry in $profiles.GetEnumerator()) {
            $profileName = $entry.Key
            $provider = $entry.Value.Provider
            $selection = $provider, '--profile', $profileName
            $consent = $entry.Value['Consent']
            if ($null -ne $consent) {
                if (-not $consent.Accepted) {
                    $matrixFailures.Add("$profileName is BLOCKED. Supply $($consent.ScriptFlag) for this run.")
                    continue
                }
                $selection += $consent.RunnerFlag
            }
            try {
                $expectedFacts = @{ $provider = $entry.Value.Facts }
                Invoke-DriverMatrix -Python $python -Selection $selection -ExpectedProfiles @{ $provider = $profileName } -ExpectedFacts $expectedFacts
                Write-Host "PASS W002 $profileName compatibility suite"
            }
            catch {
                $matrixFailures.Add("$profileName failed. $($_.Exception.Message)")
            }
        }
        try {
            Invoke-SqliteCompatibility -Python $python
        }
        catch {
            $matrixFailures.Add($_.Exception.Message)
        }
    }
    if ($matrixFailures.Count -gt 0) {
        throw ($matrixFailures -join [Environment]::NewLine)
    }
    if ($Compatibility) {
        # SQLite file compatibility reruns the baseline profile and adds one linked runtime.
        $distinctProfiles = $baselineProfiles.Count + $profiles.Count + 1
        Write-Host "PASS W002 compatibility matrix with $distinctProfiles distinct engine profiles"
    }
}
finally {
    $env:GOTOOLCHAIN = $savedToolchain
    $env:GOWORK = $savedWorkspace
    $env:GOEF_DRIVERLAB_PROVIDER = $savedProvider
    Pop-Location
}
