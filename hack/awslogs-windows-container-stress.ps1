<#
.SYNOPSIS
    Runs the containerized awslogs Windows runtime hot-stress probe.
.DESCRIPTION
    Builds awslogs.test.exe inside the prepared Moby Windows test image and runs
    TestNewStreamConfig in four concurrent processes for 25 rounds. Each process
    runs the test 1,000 times, for 100,000 planned top-level test executions.

    The script stops on the first child timeout, nonzero exit, or fatal runtime
    output. Windows Error Reporting is configured for one full dump while the
    probe runs. Crash dumps can contain sensitive process memory and must be
    protected before sharing.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ArtifactDirectory,
    [Parameter(Mandatory = $true)][string]$ExpectedGoVersion,
    [Parameter(Mandatory = $true)][int]$Rounds,
    [Parameter(Mandatory = $true)][int]$Workers,
    [Parameter(Mandatory = $true)][int]$TestCount,
    [Parameter(Mandatory = $true)][int]$ChildTimeoutSeconds
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
if (Get-Variable PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

function Test-FatalRuntimeOutput {
    param(
        [Parameter(Mandatory = $true)][string]$StandardOutput,
        [Parameter(Mandatory = $true)][string]$StandardError,
        [Parameter(Mandatory = $true)][string]$Pattern
    )

    foreach ($path in @($StandardOutput, $StandardError)) {
        if ((Test-Path -LiteralPath $path -PathType Leaf) -and
            (Select-String -LiteralPath $path -Pattern $Pattern -Quiet -ErrorAction SilentlyContinue)) {
            return $true
        }
    }
    return $false
}

function Stop-TestProcess {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process,
        [Parameter(Mandatory = $true)][string]$TaskkillPath
    )

    try {
        $Process.Refresh()
        if ($Process.HasExited) {
            return ''
        }
    }
    catch {
        return $_.Exception.Message
    }

    $errors = @()
    try {
        & $TaskkillPath /PID $Process.Id /T /F *> $null
        if ($LASTEXITCODE -ne 0) {
            $errors += "taskkill exited with code $LASTEXITCODE"
        }
    }
    catch {
        $errors += "taskkill failed: $($_.Exception.Message)"
    }

    try {
        $Process.Refresh()
        if (-not $Process.HasExited -and -not $Process.WaitForExit(5000)) {
            $Process.Kill()
            $Process.WaitForExit(5000) | Out-Null
        }
    }
    catch {
        $errors += "process termination failed: $($_.Exception.Message)"
    }
    return ($errors -join '; ')
}

function Get-ExitCodeHex {
    param($ExitCode)

    if ($ExitCode -isnot [int]) {
        return 'unknown'
    }
    return '0x{0:x8}' -f [BitConverter]::ToUInt32(
        [BitConverter]::GetBytes([int]$ExitCode), 0)
}

function Get-ExitedProcessExitCode {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process
    )

    $Process.WaitForExit()
    $Process.Refresh()
    if (-not $Process.HasExited) {
        throw "process $($Process.Id) was not exited after waiting for it"
    }
    $exitCode = $Process.ExitCode
    if ($exitCode -isnot [int]) {
        throw "exit code is unavailable for exited process $($Process.Id): '$exitCode'"
    }
    return [int]$exitCode
}

function Wait-DumpStability {
    param(
        [Parameter(Mandatory = $true)][string]$DumpDirectory,
        [Parameter(Mandatory = $true)][bool]$WaitForAppearance
    )

    $lastSignature = ''
    $stableChecks = 0
    $dumpSeen = $false
    for ($check = 0; $check -lt 60; $check++) {
        $dumps = @(Get-ChildItem -LiteralPath $DumpDirectory -Filter '*.dmp' -File `
            -ErrorAction SilentlyContinue | Sort-Object Name)
        if ($dumps.Count -gt 0) {
            $dumpSeen = $true
            $signature = ($dumps | ForEach-Object { "$($_.Name):$($_.Length)" }) -join '|'
            if ($signature -eq $lastSignature) {
                $stableChecks++
            }
            else {
                $lastSignature = $signature
                $stableChecks = 1
            }
            if ($stableChecks -ge 3) {
                return [pscustomobject]@{
                    Status = 'stable'
                    Count = $dumps.Count
                    Signature = $signature
                }
            }
        }
        elseif (-not $WaitForAppearance) {
            break
        }
        if ($check -lt 59) {
            Start-Sleep -Seconds 1
        }
    }

    return [pscustomobject]@{
        Status = if ($dumpSeen) { 'stabilization-timeout' } else { 'no-dump' }
        Count = if ($dumpSeen) { $dumps.Count } else { 0 }
        Signature = $lastSignature
    }
}

function Add-SummaryRow {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)]$State,
        [Parameter(Mandatory = $true)][string]$Status,
        [Parameter(Mandatory = $true)]$ExitCode,
        [Parameter(Mandatory = $true)][bool]$TimedOut,
        [Parameter(Mandatory = $true)][bool]$FatalOutput
    )

    $duration = [long]([DateTime]::UtcNow - $State.StartedUtc).TotalMilliseconds
    $exitCodeHex = Get-ExitCodeHex -ExitCode $ExitCode
    ('{0},{1},{2},{3},{4},{5},{6},{7},{8},{9},{10}' -f
        $State.Round, $State.Worker, $State.StartedUtc.ToString('o'), $duration,
        $State.Process.Id, $ExitCode, $exitCodeHex,
        $TimedOut.ToString().ToLowerInvariant(),
        $FatalOutput.ToString().ToLowerInvariant(), $Status, $TestCount) |
        Add-Content -LiteralPath $Path -Encoding ascii
}

$artifactReady = $false
$scriptExitCode = 1
$resultSummary = 'setup did not complete'
$phase = 'validating fixed stress parameters'
$activeProcesses = @()
$completedWorkerProcesses = 0
$failedRound = 0
$failedWorker = 0
$failureClassification = ''
$failureExitCode = 'unknown'
$stressStarted = $false
$stressFailed = $false
$werConfigured = $false
$werKeyCreated = $false
$werRootCreated = $false
$werCleanupStatus = 'not-required'
$werNotes = @(
    'warning=Crash dumps may contain sensitive process memory; protect them before sharing.'
)
$dumpStatus = 'not-checked'
$dumpCount = 0
$dumpSignature = ''
$tracebackWasSet = $null -ne (Get-Item Env:\GOTRACEBACK -ErrorAction SilentlyContinue)
$originalTraceback = $env:GOTRACEBACK
$tempDirectory = ''
$testExecutable = ''
$taskkillPath = ''
$werRoot = 'HKLM:\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps'
$werKey = "$werRoot\awslogs.test.exe"

try {
    $artifactPath = [IO.Path]::GetFullPath($ArtifactDirectory)
    New-Item -ItemType Directory -Path $artifactPath -Force | Out-Null
    $artifactPath = (Get-Item -LiteralPath $artifactPath).FullName
    $artifactReady = $true
    $dumpDirectory = Join-Path $artifactPath 'dumps'
    New-Item -ItemType Directory -Path $dumpDirectory -Force | Out-Null

    if ($Rounds -ne 25 -or $Workers -ne 4 -or $TestCount -ne 1000 -or
        $ChildTimeoutSeconds -ne 130) {
        throw 'fixed stress parameters must be rounds=25, workers=4, test_count=1000, child_timeout_seconds=130'
    }
    if ($ExpectedGoVersion -notmatch '^1\.26\.(?:[3-9]|[1-9][0-9])$') {
        throw "invalid expected Go version '$ExpectedGoVersion'"
    }

    $plannedExecutions = $Rounds * $Workers * $TestCount
    @(
        "expected_go_version=$ExpectedGoVersion"
        "rounds=$Rounds"
        "workers_per_round=$Workers"
        "test_count_per_worker=$TestCount"
        "planned_top_level_executions=$plannedExecutions"
        'test_run=^TestNewStreamConfig$'
        'go_test_timeout=2m'
        "parent_child_timeout_seconds=$ChildTimeoutSeconds"
        'successful_process_logs_retained=false'
        'warning=Crash dumps may contain sensitive process memory; protect them before sharing.'
    ) | Set-Content -LiteralPath (Join-Path $artifactPath 'container-stress-parameters.txt') -Encoding utf8

    $phase = 'validating the Windows amd64 container'
    if ($env:OS -ne 'Windows_NT' -or -not [Environment]::Is64BitOperatingSystem -or
        -not [Environment]::Is64BitProcess -or $env:PROCESSOR_ARCHITECTURE -ne 'AMD64') {
        throw 'a native Windows amd64 container with 64-bit PowerShell is required'
    }
    if ($env:FROM_DOCKERFILE -ne '1') {
        throw 'the probe must run in the image prepared by Dockerfile.windows'
    }
    if ($env:GOTOOLCHAIN -ne 'local') {
        throw "GOTOOLCHAIN must be local, found '$env:GOTOOLCHAIN'"
    }

    $goCommand = Get-Command go.exe -CommandType Application -ErrorAction SilentlyContinue |
        Select-Object -First 1
    $systeminfoCommand = Get-Command systeminfo.exe -CommandType Application -ErrorAction SilentlyContinue |
        Select-Object -First 1
    $taskkillCommand = Get-Command taskkill.exe -CommandType Application -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if ($null -eq $goCommand -or $null -eq $systeminfoCommand -or $null -eq $taskkillCommand) {
        throw 'go.exe, systeminfo.exe, and taskkill.exe must be available in PATH'
    }
    $goPath = $goCommand.Source
    $systeminfoPath = $systeminfoCommand.Source
    $taskkillPath = $taskkillCommand.Source

    @(
        "computer_name=$env:COMPUTERNAME"
        "os=$env:OS"
        "processor_architecture=$env:PROCESSOR_ARCHITECTURE"
        "number_of_processors=$env:NUMBER_OF_PROCESSORS"
        "powershell_version=$($PSVersionTable.PSVersion)"
        "is_64_bit_process=$([Environment]::Is64BitProcess)"
        "from_dockerfile=$env:FROM_DOCKERFILE"
        "gotoolchain=$env:GOTOOLCHAIN"
        "gotraceback=crash"
    ) | Set-Content -LiteralPath (Join-Path $artifactPath 'container-metadata.txt') -Encoding utf8

    $phase = 'recording container system and Go metadata'
    & $systeminfoPath *> (Join-Path $artifactPath 'container-systeminfo.txt')
    if ($LASTEXITCODE -ne 0) {
        throw "systeminfo.exe failed with exit code $LASTEXITCODE"
    }
    & $goPath version *> (Join-Path $artifactPath 'container-go-version.txt')
    if ($LASTEXITCODE -ne 0) {
        throw "go version failed with exit code $LASTEXITCODE"
    }
    $goVersionOutput = (Get-Content -LiteralPath (Join-Path $artifactPath 'container-go-version.txt') -Raw).Trim()
    $expectedVersionPattern = '^go version go' + [Regex]::Escape($ExpectedGoVersion) + ' windows/amd64$'
    if ($goVersionOutput -notmatch $expectedVersionPattern) {
        throw "expected Go $ExpectedGoVersion for windows/amd64, found '$goVersionOutput'"
    }
    & $goPath env *> (Join-Path $artifactPath 'container-go-env.txt')
    if ($LASTEXITCODE -ne 0) {
        throw "go env failed with exit code $LASTEXITCODE"
    }
    $platformOutput = @(& $goPath env GOHOSTOS GOHOSTARCH GOOS GOARCH GOTOOLCHAIN 2>&1 |
        ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
    $platformOutput | Set-Content -LiteralPath (Join-Path $artifactPath 'container-go-platform.txt') -Encoding utf8
    if ($LASTEXITCODE -ne 0 -or $platformOutput.Count -ne 5 -or
        $platformOutput[0] -ne 'windows' -or $platformOutput[1] -ne 'amd64' -or
        $platformOutput[2] -ne 'windows' -or $platformOutput[3] -ne 'amd64' -or
        $platformOutput[4] -ne 'local') {
        throw "Go must use windows/amd64 with GOTOOLCHAIN=local; got '$($platformOutput -join '/')'"
    }

    $repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
    $packageDirectory = Join-Path $repoRoot 'daemon\logger\awslogs'
    if (-not (Test-Path -LiteralPath (Join-Path $repoRoot 'go.mod') -PathType Leaf) -or
        -not (Test-Path -LiteralPath $packageDirectory -PathType Container)) {
        throw "the Moby source and awslogs package are required under '$repoRoot'"
    }

    $phase = 'building awslogs.test.exe inside the container'
    $tempDirectory = Join-Path $env:TEMP "awslogs-container-stress-$PID-$([Guid]::NewGuid().ToString('N'))"
    New-Item -ItemType Directory -Path $tempDirectory | Out-Null
    $testExecutable = Join-Path $tempDirectory 'awslogs.test.exe'
    $buildArguments = @(
        'test', '-c', '-a', '-cover', '-covermode=atomic', '-ldflags=-w',
        '-o', $testExecutable, './daemon/logger/awslogs'
    )
    $buildCommand = 'go test -c -a -cover -covermode=atomic -ldflags=-w -o "' +
        $testExecutable + '" ./daemon/logger/awslogs'
    $buildCommand | Set-Content -LiteralPath (Join-Path $artifactPath 'container-build-command.txt') -Encoding utf8
    Push-Location -LiteralPath $repoRoot
    try {
        & $goPath @buildArguments *> (Join-Path $artifactPath 'container-build-output.txt')
        $buildExitCode = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
    "exit_code=$buildExitCode" |
        Set-Content -LiteralPath (Join-Path $artifactPath 'container-build-result.txt') -Encoding utf8
    if ($buildExitCode -ne 0) {
        throw "go test -c failed with exit code $buildExitCode"
    }
    if (-not (Test-Path -LiteralPath $testExecutable -PathType Leaf) -or
        (Get-Item -LiteralPath $testExecutable).Length -eq 0) {
        throw "go test -c did not produce '$testExecutable'"
    }

    $artifactExecutable = Join-Path $artifactPath 'awslogs.test.exe'
    Copy-Item -LiteralPath $testExecutable -Destination $artifactExecutable
    $builtHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $testExecutable).Hash.ToLowerInvariant()
    $artifactHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $artifactExecutable).Hash.ToLowerInvariant()
    if ($builtHash -ne $artifactHash) {
        throw 'the artifact copy of awslogs.test.exe does not match the executable that will be run'
    }
    "$artifactHash  awslogs.test.exe" |
        Set-Content -LiteralPath (Join-Path $artifactPath 'container-executable.sha256') -Encoding ascii
    $buildInfoPath = Join-Path $artifactPath 'container-executable-build-info.txt'
    & $goPath version -m $testExecutable *> $buildInfoPath
    if ($LASTEXITCODE -ne 0) {
        throw "go version -m failed with exit code $LASTEXITCODE"
    }

    $phase = 'configuring Windows Error Reporting LocalDumps'
    try {
        if (-not (Test-Path -LiteralPath $werRoot)) {
            New-Item -Path $werRoot -Force | Out-Null
            $werRootCreated = $true
        }
        if (Test-Path -LiteralPath $werKey) {
            throw "LocalDumps key '$werKey' already exists"
        }
        New-Item -Path $werKey | Out-Null
        $werKeyCreated = $true
        New-ItemProperty -LiteralPath $werKey -Name DumpFolder -PropertyType ExpandString `
            -Value $dumpDirectory | Out-Null
        New-ItemProperty -LiteralPath $werKey -Name DumpType -PropertyType DWord -Value 2 | Out-Null
        New-ItemProperty -LiteralPath $werKey -Name DumpCount -PropertyType DWord -Value 1 | Out-Null
        $werConfiguration = Get-ItemProperty -LiteralPath $werKey
        if (-not $werConfiguration.DumpFolder.Equals($dumpDirectory, [StringComparison]::OrdinalIgnoreCase) -or
            [int]$werConfiguration.DumpType -ne 2 -or [int]$werConfiguration.DumpCount -ne 1) {
            throw 'LocalDumps values could not be verified after configuration'
        }
        $werConfigured = $true
        $werNotes += @(
            'configured=true'
            "key=$werKey"
            "dump_folder=$dumpDirectory"
            'dump_type=2'
            'dump_count=1'
        )
        Write-Warning 'WER full dumps may contain sensitive process memory; protect artifacts before sharing.'
    }
    catch {
        $werNotes += "configured=false; error=$($_.Exception.Message)"
        throw "Windows Error Reporting LocalDumps setup is unavailable: $($_.Exception.Message)"
    }

    $phase = 'running the fixed TestNewStreamConfig hot-stress probe'
    $summaryPath = Join-Path $artifactPath 'container-round-worker-summary.csv'
    'round,worker,started_utc,duration_ms,process_id,exit_code,exit_code_hex,timed_out,fatal_output,status,test_count' |
        Set-Content -LiteralPath $summaryPath -Encoding ascii
    $testArguments = @(
        '-test.run=^TestNewStreamConfig$',
        "-test.count=$TestCount",
        '-test.timeout=2m'
    )
    ('"' + $testExecutable + '" ' + ($testArguments -join ' ')) |
        Set-Content -LiteralPath (Join-Path $artifactPath 'container-test-command.txt') -Encoding utf8
    $fatalPattern = '(?i)(fatal error:|runtime: unexpected|runtime: out of memory|access violation|exception 0xc0000005)'
    $env:GOTRACEBACK = 'crash'
    $stressStarted = $true

    for ($round = 1; $round -le $Rounds; $round++) {
        $activeProcesses = @()
        for ($worker = 1; $worker -le $Workers; $worker++) {
            $stdoutPath = Join-Path $tempDirectory ("round-{0:D2}-worker-{1}.stdout.log" -f $round, $worker)
            $stderrPath = Join-Path $tempDirectory ("round-{0:D2}-worker-{1}.stderr.log" -f $round, $worker)
            $startedUtc = [DateTime]::UtcNow
            $process = Start-Process -FilePath $testExecutable -ArgumentList $testArguments `
                -WorkingDirectory $repoRoot -NoNewWindow -PassThru `
                -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
            $activeProcesses += [pscustomobject]@{
                Round = $round
                Worker = $worker
                StartedUtc = $startedUtc
                DeadlineUtc = $startedUtc.AddSeconds($ChildTimeoutSeconds)
                Process = $process
                StandardOutput = $stdoutPath
                StandardError = $stderrPath
                Terminated = $false
                TerminationError = ''
                ExitCode = 'unknown'
            }
        }

        $firstFailure = $null
        while ($null -eq $firstFailure) {
            $allExited = $true
            foreach ($state in $activeProcesses) {
                $state.Process.Refresh()
                $hasExited = $state.Process.HasExited
                if ($hasExited) {
                    $state.ExitCode = Get-ExitedProcessExitCode -Process $state.Process
                }
                else {
                    $allExited = $false
                }

                $fatalOutput = Test-FatalRuntimeOutput `
                    -StandardOutput $state.StandardOutput -StandardError $state.StandardError `
                    -Pattern $fatalPattern
                if ($fatalOutput) {
                    $firstFailure = [pscustomobject]@{
                        State = $state
                        Classification = 'fatal-runtime'
                        TimedOut = $false
                    }
                    break
                }
                if (-not $hasExited -and [DateTime]::UtcNow -ge $state.DeadlineUtc) {
                    $firstFailure = [pscustomobject]@{
                        State = $state
                        Classification = 'timeout'
                        TimedOut = $true
                    }
                    break
                }
                if ($hasExited -and $state.ExitCode -ne 0) {
                    $firstFailure = [pscustomobject]@{
                        State = $state
                        Classification = 'nonzero'
                        TimedOut = $false
                    }
                    break
                }
            }
            if ($null -ne $firstFailure -or $allExited) {
                break
            }
            Start-Sleep -Milliseconds 200
        }

        if ($null -ne $firstFailure) {
            foreach ($state in $activeProcesses) {
                $state.Process.Refresh()
                $isFatalProcess = $firstFailure.Classification -eq 'fatal-runtime' -and
                    $state.Worker -eq $firstFailure.State.Worker
                if ($isFatalProcess) {
                    continue
                }
                if (-not $state.Process.HasExited) {
                    $state.Terminated = $true
                    $state.TerminationError = Stop-TestProcess `
                        -Process $state.Process -TaskkillPath $taskkillPath
                }
            }

            # Give WER a bounded opportunity to observe a fatal child before
            # forcing it down. Sibling workers have already been terminated.
            if ($firstFailure.Classification -eq 'fatal-runtime') {
                $fatalState = $firstFailure.State
                $fatalState.Process.Refresh()
                if (-not $fatalState.Process.HasExited) {
                    $remainingMilliseconds = [int][Math]::Max(0,
                        ($fatalState.DeadlineUtc - [DateTime]::UtcNow).TotalMilliseconds)
                    if ($remainingMilliseconds -gt 0) {
                        $fatalState.Process.WaitForExit($remainingMilliseconds) | Out-Null
                    }
                    $fatalState.Process.Refresh()
                    if (-not $fatalState.Process.HasExited) {
                        $fatalState.Terminated = $true
                        $fatalState.TerminationError = Stop-TestProcess `
                            -Process $fatalState.Process -TaskkillPath $taskkillPath
                    }
                }
            }

            foreach ($state in $activeProcesses) {
                $state.Process.Refresh()
                $hasExited = $state.Process.HasExited
                if ($hasExited) {
                    $state.ExitCode = Get-ExitedProcessExitCode -Process $state.Process
                    $childExitCode = $state.ExitCode
                }
                else {
                    $childExitCode = 'unknown'
                }
                $fatalOutput = Test-FatalRuntimeOutput `
                    -StandardOutput $state.StandardOutput -StandardError $state.StandardError `
                    -Pattern $fatalPattern

                if ($state.Worker -eq $firstFailure.State.Worker) {
                    $status = $firstFailure.Classification
                    $timedOut = $firstFailure.TimedOut
                    $failureExitCode = $childExitCode
                }
                elseif ($state.Terminated) {
                    $status = 'terminated-sibling'
                    $timedOut = $false
                }
                elseif ($fatalOutput) {
                    $status = 'additional-fatal-runtime'
                    $timedOut = $false
                }
                elseif ($childExitCode -is [int] -and $childExitCode -ne 0) {
                    $status = 'additional-nonzero'
                    $timedOut = $false
                }
                else {
                    $status = 'passed'
                    $timedOut = $false
                    $completedWorkerProcesses++
                }
                Add-SummaryRow -Path $summaryPath -State $state -Status $status `
                    -ExitCode $childExitCode -TimedOut $timedOut -FatalOutput $fatalOutput
            }

            $failedRound = $round
            $failedWorker = $firstFailure.State.Worker
            $failureClassification = $firstFailure.Classification
            $failureState = $firstFailure.State
            $failureStdout = Join-Path $artifactPath 'first-failure.stdout.log'
            $failureStderr = Join-Path $artifactPath 'first-failure.stderr.log'
            Move-Item -LiteralPath $failureState.StandardOutput -Destination $failureStdout -Force
            Move-Item -LiteralPath $failureState.StandardError -Destination $failureStderr -Force
            $failureFatalOutput = Test-FatalRuntimeOutput `
                -StandardOutput $failureStdout -StandardError $failureStderr `
                -Pattern $fatalPattern
            @(
                '===== stdout ====='
                (Get-Content -LiteralPath $failureStdout -Raw)
                '===== stderr ====='
                (Get-Content -LiteralPath $failureStderr -Raw)
            ) | Set-Content -LiteralPath (Join-Path $artifactPath 'first-failure-output.txt') -Encoding utf8
            @(
                "round=$failedRound"
                "worker=$failedWorker"
                "classification=$failureClassification"
                "exit_code=$failureExitCode"
                "exit_code_hex=$(Get-ExitCodeHex -ExitCode $failureExitCode)"
                "timed_out=$($firstFailure.TimedOut.ToString().ToLowerInvariant())"
                "fatal_output=$($failureFatalOutput.ToString().ToLowerInvariant())"
                "process_id=$($failureState.Process.Id)"
                "started_utc=$($failureState.StartedUtc.ToString('o'))"
                "termination_error=$($failureState.TerminationError)"
                'dump_status=pending-final-stability-check'
            ) | Set-Content -LiteralPath (Join-Path $artifactPath 'first-failure.txt') -Encoding utf8

            foreach ($state in $activeProcesses) {
                Remove-Item -LiteralPath $state.StandardOutput, $state.StandardError `
                    -Force -ErrorAction SilentlyContinue
                $state.Process.Dispose()
            }
            $activeProcesses = @()
            $stressFailed = $true
            $resultSummary = "failed in round $failedRound worker $failedWorker ($failureClassification)"
            Write-Host -ForegroundColor Red "Failure in round $failedRound worker ${failedWorker}: $failureClassification (exit $failureExitCode)"
            break
        }

        foreach ($state in $activeProcesses) {
            $state.Process.Refresh()
            $state.ExitCode = Get-ExitedProcessExitCode -Process $state.Process
            $childExitCode = $state.ExitCode
            $fatalOutput = Test-FatalRuntimeOutput `
                -StandardOutput $state.StandardOutput -StandardError $state.StandardError `
                -Pattern $fatalPattern
            Add-SummaryRow -Path $summaryPath -State $state -Status 'passed' `
                -ExitCode $childExitCode -TimedOut $false -FatalOutput $fatalOutput
            $completedWorkerProcesses++
            Remove-Item -LiteralPath $state.StandardOutput, $state.StandardError -Force
            $state.Process.Dispose()
        }
        $activeProcesses = @()
        Write-Host "Completed round $round/$Rounds ($($round * $Workers * $TestCount) top-level executions)"
    }

    if (-not $stressFailed) {
        $scriptExitCode = 0
        $resultSummary = "all $plannedExecutions planned top-level executions passed"
    }
}
catch {
    $scriptExitCode = 1
    $resultSummary = "$phase failed: $($_.Exception.Message)"
    if ($artifactReady) {
        @(
            "phase=$phase"
            "exception_type=$($_.Exception.GetType().FullName)"
            "message=$($_.Exception.Message)"
            "position=$($_.InvocationInfo.PositionMessage)"
        ) | Set-Content -LiteralPath (Join-Path $artifactPath 'container-script-error.txt') -Encoding utf8
    }
    Write-Host -ForegroundColor Red "ERROR: $resultSummary"
}
finally {
    foreach ($state in $activeProcesses) {
        try {
            $state.Process.Refresh()
            if (-not $state.Process.HasExited) {
                $state.Terminated = $true
                $state.TerminationError = Stop-TestProcess `
                    -Process $state.Process -TaskkillPath $taskkillPath
            }
            $state.Process.Dispose()
        }
        catch { }
    }
    $activeProcesses = @()

    try {
        if ($tracebackWasSet) {
            $env:GOTRACEBACK = $originalTraceback
        }
        else {
            Remove-Item Env:\GOTRACEBACK -ErrorAction SilentlyContinue
        }
    }
    catch {
        $scriptExitCode = 1
        $resultSummary = "failed to restore GOTRACEBACK: $($_.Exception.Message)"
    }

    if ($artifactReady -and (Test-Path -LiteralPath $dumpDirectory -PathType Container)) {
        try {
            $dumpResult = Wait-DumpStability -DumpDirectory $dumpDirectory `
                -WaitForAppearance ($stressStarted -and $scriptExitCode -ne 0 -and $werConfigured)
            $dumpStatus = $dumpResult.Status
            $dumpCount = $dumpResult.Count
            $dumpSignature = $dumpResult.Signature
            if ($dumpStatus -eq 'stabilization-timeout' -and $scriptExitCode -eq 0) {
                $scriptExitCode = 1
                $resultSummary = 'an unexpected dump did not stabilize before container exit'
            }
        }
        catch {
            $dumpStatus = "stability-check-error: $($_.Exception.Message)"
            $scriptExitCode = 1
            $resultSummary = 'dump stabilization check failed'
        }
    }

    if ($werKeyCreated) {
        try {
            if (Test-Path -LiteralPath $werKey) {
                Remove-Item -LiteralPath $werKey -Recurse -Force
            }
            $werCleanupStatus = 'executable-key-removed'
        }
        catch {
            $werCleanupStatus = "executable-key-removal-failed: $($_.Exception.Message)"
            $scriptExitCode = 1
            $resultSummary = 'Windows Error Reporting cleanup failed'
        }
    }
    if ($werRootCreated) {
        try {
            if (Test-Path -LiteralPath $werRoot) {
                $rootKey = Get-Item -LiteralPath $werRoot
                if ($rootKey.SubKeyCount -eq 0 -and $rootKey.ValueCount -eq 0) {
                    Remove-Item -LiteralPath $werRoot -Force
                    $werCleanupStatus += '; empty-parent-removed'
                }
                else {
                    $werCleanupStatus += '; parent-kept-not-empty'
                }
            }
        }
        catch {
            $werCleanupStatus += "; parent-cleanup-failed: $($_.Exception.Message)"
            $scriptExitCode = 1
            $resultSummary = 'Windows Error Reporting cleanup failed'
        }
    }

    if ($artifactReady) {
        try {
            $werNotes += @(
                "cleanup_status=$werCleanupStatus"
                "dump_status=$dumpStatus"
                "dump_count=$dumpCount"
                "dump_signature=$dumpSignature"
            )
            $werNotes | Set-Content -LiteralPath (Join-Path $artifactPath 'container-wer-localdumps.txt') -Encoding utf8
            @(
                "status=$dumpStatus"
                "count=$dumpCount"
                "signature=$dumpSignature"
                'warning=Crash dumps may contain sensitive process memory; protect them before sharing.'
            ) | Set-Content -LiteralPath (Join-Path $artifactPath 'container-dump-status.txt') -Encoding utf8
            if ($failedRound -gt 0) {
                Add-Content -LiteralPath (Join-Path $artifactPath 'first-failure.txt') `
                    -Value "final_dump_status=$dumpStatus`nfinal_dump_count=$dumpCount" -Encoding utf8
            }

            $confirmedExecutions = $completedWorkerProcesses * $TestCount
            @(
                "exit_code=$scriptExitCode"
                "result=$resultSummary"
                "stress_started=$($stressStarted.ToString().ToLowerInvariant())"
                "rounds_planned=$Rounds"
                "workers_per_round=$Workers"
                "test_count_per_worker=$TestCount"
                "planned_top_level_executions=$($Rounds * $Workers * $TestCount)"
                "completed_worker_processes=$completedWorkerProcesses"
                "confirmed_completed_top_level_executions=$confirmedExecutions"
                "failed_round=$failedRound"
                "failed_worker=$failedWorker"
                "failure_classification=$failureClassification"
                "failure_exit_code=$failureExitCode"
                "wer_configured=$($werConfigured.ToString().ToLowerInvariant())"
                "wer_cleanup_status=$werCleanupStatus"
                "dump_status=$dumpStatus"
                "finished_utc=$([DateTime]::UtcNow.ToString('o'))"
            ) | Set-Content -LiteralPath (Join-Path $artifactPath 'container-result.txt') -Encoding utf8
        }
        catch {
            $scriptExitCode = 1
            $resultSummary = "could not finalize container artifacts: $($_.Exception.Message)"
            try {
                @(
                    "exit_code=$scriptExitCode"
                    "result=$resultSummary"
                ) | Set-Content -LiteralPath (Join-Path $artifactPath 'container-finalization-error.txt') -Encoding utf8
            }
            catch { }
        }
    }

    if ($tempDirectory -and (Test-Path -LiteralPath $tempDirectory)) {
        Remove-Item -LiteralPath $tempDirectory -Recurse -Force -ErrorAction SilentlyContinue
    }
    if ($scriptExitCode -eq 0) {
        Write-Host -ForegroundColor Green "PASS: $resultSummary"
    }
    else {
        Write-Host -ForegroundColor Red "FAIL: $resultSummary"
    }
    if ($artifactReady) {
        Write-Host "Artifacts: $artifactPath"
    }
}

exit $scriptExitCode
