<#
.SYNOPSIS
    Repeatedly runs the awslogs TestNewStreamConfig crash investigation on Windows.
.DESCRIPTION
    Builds a covered, stripped awslogs test executable and starts a fresh process for
    every TestNewStreamConfig iteration. It stops at the first timeout, nonzero exit,
    or fatal runtime output and saves the useful diagnostics.
    Run with 64-bit PowerShell on native Windows amd64. An elevated run enables one
    full Windows Error Reporting LocalDumps entry for the unique test executable.
    Crash dumps can contain sensitive process memory; protect artifacts before sharing.
.PARAMETER Iterations
    Number of fresh test processes. The default is 250; valid values are 1 through 1000.
.PARAMETER ArtifactDirectory
    Directory for the executable, logs, dumps, metadata, and checksums. Relative paths
    are resolved from the caller's working directory. Without this option, a timestamped
    directory is created under awslogs-crash-artifacts in the checkout.
.PARAMETER KeepSuccessfulLogs
    Keep stdout and stderr from successful iterations. They are removed by default.
.EXAMPLE
    PS C:\src\moby> .\hack\awslogs-windows-crash.ps1
    Runs 250 iterations and creates a timestamped artifact directory.
.EXAMPLE
    PS C:\> & C:\src\moby\hack\awslogs-windows-crash.ps1 -Iterations 500 -ArtifactDirectory C:\crash-data\awslogs
    Runs from outside the checkout and writes artifacts to the given directory.
.EXAMPLE
    PS C:\src\moby> .\hack\awslogs-windows-crash.ps1 -Iterations 25 -KeepSuccessfulLogs
    Retains stdout and stderr for successful iterations.
#>
[CmdletBinding()]
param(
    [ValidateRange(1, 1000)][int]$Iterations = 250,
    [string]$ArtifactDirectory,
    [switch]$KeepSuccessfulLogs
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
if (Get-Variable PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}
function Stop-TestProcess {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process,
        [string]$TaskkillPath,
        [string]$LogDirectory,
        [int]$Iteration
    )
    try { $Process.Refresh(); if ($Process.HasExited) { return } }
    catch { return }
    if ($TaskkillPath) {
        $taskkillOut = Join-Path $LogDirectory ("iteration-{0:D4}.taskkill.stdout.log" -f $Iteration)
        $taskkillErr = Join-Path $LogDirectory ("iteration-{0:D4}.taskkill.stderr.log" -f $Iteration)
        try {
            $killer = Start-Process -FilePath $TaskkillPath -ArgumentList @('/PID', $Process.Id, '/T', '/F') `
                -NoNewWindow -PassThru -RedirectStandardOutput $taskkillOut -RedirectStandardError $taskkillErr
            if (-not $killer.WaitForExit(10000)) { $killer.Kill() } else { $killer.WaitForExit() }
            $killer.Dispose()
        } catch { }
    }
    try {
        $Process.Refresh()
        if (-not $Process.HasExited -and -not $Process.WaitForExit(5000)) { $Process.Kill(); $Process.WaitForExit(5000) | Out-Null }
    } catch { }
}
function Write-ArtifactChecksums {
    param([Parameter(Mandatory = $true)][string]$Root, [Parameter(Mandatory = $true)][string]$OutputPath)
    $prefix = [IO.Path]::GetFullPath($Root).TrimEnd([char[]]@('\', '/')) + '\'
    $lines = foreach ($file in Get-ChildItem -LiteralPath $Root -Recurse -File | Sort-Object FullName) {
        if ($file.FullName.Equals($OutputPath, [StringComparison]::OrdinalIgnoreCase)) { continue }
        $relative = $file.FullName.Substring($prefix.Length).Replace('\', '/')
        "$((Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant())  $relative"
    }
    $lines | Set-Content -LiteralPath $OutputPath -Encoding ascii
}
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$callerLocation = Get-Location
if ($callerLocation.Provider.Name -ne 'FileSystem') { throw 'the script must be invoked from a filesystem working directory' }
$runTimestamp = [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss-fff')
if ([string]::IsNullOrWhiteSpace($ArtifactDirectory)) { $artifactPath = Join-Path $repoRoot (Join-Path 'awslogs-crash-artifacts' $runTimestamp) }
elseif ([IO.Path]::IsPathRooted($ArtifactDirectory)) { $artifactPath = [IO.Path]::GetFullPath($ArtifactDirectory) }
else { $artifactPath = [IO.Path]::GetFullPath((Join-Path $callerLocation.Path $ArtifactDirectory)) }
$artifactReady = $false; $scriptExitCode = 1; $resultSummary = 'setup did not complete'; $phase = 'creating artifact directory'
$completedIterations = 0; $failedIteration = 0; $activeIteration = 0; $activeProcess = $null; $taskkillPath = ''
$logDirectory = Join-Path $artifactPath 'logs'; $dumpDirectory = Join-Path $artifactPath 'dumps'
$werRoot = 'HKLM:\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps'; $werRootCreated = $false; $werKeyCreated = $false; $werEnabled = $false; $werNotes = @(); $resultPath = Join-Path $artifactPath 'result.txt'; $checksumPath = Join-Path $artifactPath 'checksums.sha256'
$tracebackWasSet = $null -ne (Get-Item Env:\GOTRACEBACK -ErrorAction SilentlyContinue); $originalTraceback = $env:GOTRACEBACK
try {
    New-Item -ItemType Directory -Path $artifactPath -Force | Out-Null
    $artifactPath = (Get-Item -LiteralPath $artifactPath).FullName; $artifactReady = $true
    New-Item -ItemType Directory -Path $logDirectory, $dumpDirectory -Force | Out-Null
    $phase = 'validating Windows amd64 host'
    if ($env:OS -ne 'Windows_NT' -or -not [Environment]::Is64BitOperatingSystem -or -not [Environment]::Is64BitProcess -or $env:PROCESSOR_ARCHITECTURE -ne 'AMD64') { throw 'native Windows amd64 with 64-bit PowerShell is required' }
    $phase = 'validating commands and source'
    $goCommand = Get-Command go.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    $gitCommand = Get-Command git.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    $systeminfoCommand = Get-Command systeminfo.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    $taskkillCommand = Get-Command taskkill.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $goCommand -or $null -eq $gitCommand -or $null -eq $systeminfoCommand -or
        $null -eq $taskkillCommand) {
        throw 'go.exe, git.exe, systeminfo.exe, and taskkill.exe must all be available in PATH'
    }
    $goPath = $goCommand.Source; $gitPath = $gitCommand.Source
    $systeminfoPath = $systeminfoCommand.Source; $taskkillPath = $taskkillCommand.Source
    $packageDirectory = Join-Path $repoRoot 'daemon\logger\awslogs'
    $testSource = Join-Path $packageDirectory 'cloudwatchlogs_test.go'
    if (-not (Test-Path -LiteralPath (Join-Path $repoRoot 'go.mod') -PathType Leaf) -or
        -not (Test-Path -LiteralPath $packageDirectory -PathType Container) -or
        -not (Test-Path -LiteralPath $testSource -PathType Leaf) -or
        -not (Select-String -LiteralPath $testSource -Pattern '^func TestNewStreamConfig\(' -Quiet)) {
        throw "go.mod, the awslogs package, and TestNewStreamConfig are required under '$repoRoot'"
    }
    $commandsPath = Join-Path $artifactPath 'commands.txt'
    @(
        "go=$goPath"; "git=$gitPath"; "systeminfo=$systeminfoPath"; "taskkill=$taskkillPath"
        'go version'; 'go env'; 'systeminfo'; "git -C `"$repoRoot`" rev-parse HEAD"
    ) | Set-Content -LiteralPath $commandsPath -Encoding utf8
    Push-Location -LiteralPath $repoRoot
    try {
        $phase = 'recording Go, system, and Git information'
        $goVersionPath = Join-Path $artifactPath 'go-version.txt'
        & $goPath version *> $goVersionPath
        if ($LASTEXITCODE -ne 0) { throw "go version failed with exit code $LASTEXITCODE" }
        $goEnvPath = Join-Path $artifactPath 'go-env.txt'
        & $goPath env *> $goEnvPath
        if ($LASTEXITCODE -ne 0) { throw "go env failed with exit code $LASTEXITCODE" }
        $platformPath = Join-Path $artifactPath 'go-platform.txt'
        $platformOutput = @(& $goPath env GOHOSTOS GOHOSTARCH GOOS GOARCH 2>&1)
        $platformExitCode = $LASTEXITCODE
        $platformOutput | Set-Content -LiteralPath $platformPath -Encoding utf8
        $platform = @($platformOutput | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
        if ($platformExitCode -ne 0 -or $platform.Count -ne 4 -or $platform[0] -ne 'windows' -or
            $platform[1] -ne 'amd64' -or $platform[2] -ne 'windows' -or $platform[3] -ne 'amd64') {
            throw "Go must target windows/amd64; got '$($platform -join '/')'"
        }
        $systeminfoOut = Join-Path $artifactPath 'systeminfo.txt'
        & $systeminfoPath *> $systeminfoOut
        if ($LASTEXITCODE -ne 0) { throw "systeminfo.exe failed with exit code $LASTEXITCODE" }
        $gitHeadPath = Join-Path $artifactPath 'git-head.txt'
        & $gitPath -C $repoRoot rev-parse HEAD *> $gitHeadPath
        if ($LASTEXITCODE -ne 0) { throw "git rev-parse HEAD failed with exit code $LASTEXITCODE" }
        $gitHead = (Get-Content -LiteralPath $gitHeadPath -Raw).Trim()
        if ($gitHead -notmatch '^[0-9a-fA-F]{40,64}$') { throw "git HEAD was not a commit hash: '$gitHead'" }
        $phase = 'building the covered stripped awslogs test executable'
        $executableName = "moby-awslogs-crash-$runTimestamp-$PID-$([Guid]::NewGuid().ToString('N')).test.exe"; $testExecutable = Join-Path $artifactPath $executableName
        $buildArguments = @(
            'test', '-c', '-a', '-cover', '-covermode=atomic', '-ldflags=-w',
            '-o', $testExecutable, './daemon/logger/awslogs'
        )
        $buildCommand = 'go test -c -a -cover -covermode=atomic -ldflags=-w -o "' + $testExecutable + '" ./daemon/logger/awslogs'
        Add-Content -LiteralPath $commandsPath -Value $buildCommand -Encoding utf8
        $buildOutputPath = Join-Path $artifactPath 'build-output.txt'
        & $goPath @buildArguments *> $buildOutputPath
        $buildExitCode = $LASTEXITCODE
        if ($buildExitCode -ne 0) { throw "go test -c failed with exit code $buildExitCode; see '$buildOutputPath'" }
        if (-not (Test-Path -LiteralPath $testExecutable -PathType Leaf) -or (Get-Item -LiteralPath $testExecutable).Length -eq 0) { throw "go test -c did not produce '$testExecutable'" }
        $buildInfoPath = Join-Path $artifactPath 'executable-build-info.txt'
        & $goPath version -m $testExecutable *> $buildInfoPath
        if ($LASTEXITCODE -ne 0) { throw "go version -m failed with exit code $LASTEXITCODE" }
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $testExecutable).Hash.ToLowerInvariant()
        "$hash  $executableName" | Set-Content -LiteralPath (Join-Path $artifactPath 'executable.sha256') -Encoding ascii
    }
    finally { Pop-Location }
    $werKey = Join-Path $werRoot $executableName
    $isElevated = ([Security.Principal.WindowsPrincipal]::new(
        [Security.Principal.WindowsIdentity]::GetCurrent())).IsInRole(
            [Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isElevated) {
        $werNotes += 'enabled=false; reason=PowerShell is not elevated'
        Write-Warning 'PowerShell is not elevated; continuing without Windows Error Reporting dump capture.'
    }
    else {
        try {
            if (-not (Test-Path -LiteralPath $werRoot)) {
                New-Item -Path $werRoot -Force | Out-Null
                $werRootCreated = $true
            }
            if (Test-Path -LiteralPath $werKey) {
                $werNotes += 'enabled=false; reason=unique executable registry key unexpectedly exists'
                Write-Warning "Windows Error Reporting key '$werKey' already exists; continuing without dump capture."
            }
            else {
                New-Item -Path $werKey | Out-Null
                $werKeyCreated = $true
                New-ItemProperty -LiteralPath $werKey -Name DumpFolder -PropertyType ExpandString -Value $dumpDirectory | Out-Null
                New-ItemProperty -LiteralPath $werKey -Name DumpType -PropertyType DWord -Value 2 | Out-Null
                New-ItemProperty -LiteralPath $werKey -Name DumpCount -PropertyType DWord -Value 1 | Out-Null
                $werEnabled = $true
                $werNotes += "enabled=true; key=$werKey; dump_folder=$dumpDirectory; dump_type=2; dump_count=1"
                Write-Host "Windows Error Reporting LocalDumps enabled for $executableName"
            }
        }
        catch {
            $werNotes += "enabled=false; reason=$($_.Exception.Message)"
            Write-Warning "Could not configure Windows Error Reporting; continuing without dump capture: $($_.Exception.Message)"
        }
    }
    $phase = 'running TestNewStreamConfig'
    $testArguments = @('-test.run=^TestNewStreamConfig$', '-test.count=1', '-test.timeout=2m', '-test.v')
    $testCommand = 'GOTRACEBACK=crash "' + $testExecutable + '" ' + ($testArguments -join ' ')
    Add-Content -LiteralPath $commandsPath -Value $testCommand -Encoding utf8
    $summaryPath = Join-Path $artifactPath 'iterations.csv'
    'iteration,status,duration_ms,process_id,exit_code,timed_out,fatal_output,dump_status' |
        Set-Content -LiteralPath $summaryPath -Encoding ascii
    $fatalPattern = '(?i)(fatal error:|runtime: |access violation|exception 0xc0000005)'
    for ($iteration = 1; $iteration -le $Iterations; $iteration++) {
        $activeIteration = $iteration
        $started = [DateTime]::UtcNow
        $stopwatch = [Diagnostics.Stopwatch]::StartNew()
        $stdoutPath = Join-Path $logDirectory ("iteration-{0:D4}.stdout.log" -f $iteration)
        $stderrPath = Join-Path $logDirectory ("iteration-{0:D4}.stderr.log" -f $iteration)
        Remove-Item -LiteralPath $stdoutPath, $stderrPath -Force -ErrorAction SilentlyContinue
        try {
            $env:GOTRACEBACK = 'crash'
            $activeProcess = Start-Process -FilePath $testExecutable -ArgumentList $testArguments `
                -WorkingDirectory $repoRoot -NoNewWindow -PassThru `
                -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
        }
        finally {
            if ($tracebackWasSet) { $env:GOTRACEBACK = $originalTraceback }
            else { Remove-Item Env:\GOTRACEBACK -ErrorAction SilentlyContinue }
        }
        $processID = $activeProcess.Id
        $timedOut = -not $activeProcess.WaitForExit(130000)
        if ($timedOut) {
            Stop-TestProcess -Process $activeProcess -TaskkillPath $taskkillPath `
                -LogDirectory $logDirectory -Iteration $iteration
        }
        $stopwatch.Stop()
        $activeProcess.Refresh()
        $processExited = $activeProcess.HasExited
        if (-not $processExited) {
            Stop-TestProcess -Process $activeProcess -TaskkillPath $taskkillPath `
                -LogDirectory $logDirectory -Iteration $iteration
            $activeProcess.Refresh()
            $processExited = $activeProcess.HasExited
        }
        $childExitCode = 'unknown'; $exitCodeHex = 'unknown'
        if ($processExited) {
            $activeProcess.WaitForExit()
            $childExitCode = [int]$activeProcess.ExitCode
            $exitCodeHex = '0x{0:x8}' -f [BitConverter]::ToUInt32(
                [BitConverter]::GetBytes($childExitCode), 0)
        }
        $fatalOutput = ((Test-Path -LiteralPath $stdoutPath -PathType Leaf) -and
            (Select-String -LiteralPath $stdoutPath -Pattern $fatalPattern -Quiet)) -or
            ((Test-Path -LiteralPath $stderrPath -PathType Leaf) -and
            (Select-String -LiteralPath $stderrPath -Pattern $fatalPattern -Quiet))
        if ($timedOut -or -not $processExited) { $status = 'timeout' }
        elseif ($childExitCode -ne 0) { $status = 'nonzero' }
        elseif ($fatalOutput) { $status = 'fatal-output' }
        else { $status = 'passed' }
        $dumpStatus = 'not-captured'; $dumpPath = ''; $dumpSize = 0
        if ($status -ne 'passed' -and $werEnabled) {
            $dumpStatus = 'no-dump'; $lastDump = ''; $lastSize = -1L; $stableChecks = 0
            for ($check = 0; $check -lt 30; $check++) {
                $dump = Get-ChildItem -LiteralPath $dumpDirectory -Filter '*.dmp' -File `
                    -ErrorAction SilentlyContinue | Sort-Object LastWriteTimeUtc -Descending |
                    Select-Object -First 1
                if ($null -ne $dump -and $dump.Length -gt 0) {
                    $dumpPath = $dump.FullName; $dumpSize = $dump.Length
                    if ($dumpPath -eq $lastDump -and $dumpSize -eq $lastSize) { $stableChecks++ }
                    else { $stableChecks = 1 }
                    $lastDump = $dumpPath; $lastSize = $dumpSize
                    if ($stableChecks -ge 3) { $dumpStatus = 'stable'; break }
                    $dumpStatus = 'not-stable'
                }
                if ($check -lt 29) { Start-Sleep -Seconds 1 }
            }
        }
        $activeProcess.Dispose(); $activeProcess = $null; $completedIterations = $iteration
        ('{0},{1},{2},{3},{4},{5},{6},{7}' -f $iteration, $status, $stopwatch.ElapsedMilliseconds,
            $processID, $childExitCode, $timedOut.ToString().ToLowerInvariant(),
            $fatalOutput.ToString().ToLowerInvariant(), $dumpStatus) |
            Add-Content -LiteralPath $summaryPath -Encoding ascii
        if ($status -ne 'passed') {
            $failedIteration = $iteration; $scriptExitCode = 1; $resultSummary = "failed at iteration $iteration of $Iterations ($status)"
            @(
                "iteration=$iteration"; "status=$status"; "started_utc=$($started.ToString('o'))"
                "duration_ms=$($stopwatch.ElapsedMilliseconds)"; "process_id=$processID"
                "exit_code=$childExitCode"; "exit_code_hex=$exitCodeHex"
                "timed_out=$($timedOut.ToString().ToLowerInvariant())"
                "fatal_output=$($fatalOutput.ToString().ToLowerInvariant())"
                "stdout=$stdoutPath"; "stderr=$stderrPath"; "dump_status=$dumpStatus"
                "dump_path=$dumpPath"; "dump_size=$dumpSize"
            ) | Set-Content -LiteralPath (Join-Path $artifactPath 'failure.txt') -Encoding utf8
            Write-Host -ForegroundColor Red "Failure at iteration ${iteration}: $status (exit $childExitCode [$exitCodeHex], dump $dumpStatus)"; break
        }
        if (-not $KeepSuccessfulLogs) { Remove-Item -LiteralPath $stdoutPath, $stderrPath -Force -ErrorAction SilentlyContinue }
        if (($iteration % 25) -eq 0 -or $iteration -eq $Iterations) { Write-Host "Completed $iteration/$Iterations iterations" }
    }
    if ($failedIteration -eq 0) { $scriptExitCode = 0; $resultSummary = "all $Iterations iterations passed" }
}
catch {
    $scriptExitCode = 1; $resultSummary = "$phase failed: $($_.Exception.Message)"
    if ($artifactReady) { @("phase=$phase"; "exception_type=$($_.Exception.GetType().FullName)"; "message=$($_.Exception.Message)"; "position=$($_.InvocationInfo.PositionMessage)") | Set-Content -LiteralPath (Join-Path $artifactPath 'script-error.txt') -Encoding utf8 }
    Write-Host -ForegroundColor Red "ERROR: $resultSummary"
}
finally {
    if ($null -ne $activeProcess) {
        Stop-TestProcess -Process $activeProcess -TaskkillPath $taskkillPath `
            -LogDirectory $logDirectory -Iteration $activeIteration
        try { $activeProcess.Dispose() } catch { }
        $activeProcess = $null; $scriptExitCode = 1
        $resultSummary = "interrupted while running iteration $activeIteration"
    }
    try {
        if ($tracebackWasSet) { $env:GOTRACEBACK = $originalTraceback }
        else { Remove-Item Env:\GOTRACEBACK -ErrorAction SilentlyContinue }
    }
    catch {
        $scriptExitCode = 1
        $resultSummary = "failed to restore GOTRACEBACK: $($_.Exception.Message)"
    }
    if ($werKeyCreated) {
        try {
            if (Test-Path -LiteralPath $werKey) { Remove-Item -LiteralPath $werKey -Recurse -Force }
            $werNotes += 'removed the executable LocalDumps key created by this script'
        }
        catch {
            $werNotes += "failed to remove the executable LocalDumps key: $($_.Exception.Message)"
            $scriptExitCode = 1; $resultSummary = 'Windows Error Reporting cleanup failed'
        }
    }
    if ($werRootCreated) {
        try {
            if (Test-Path -LiteralPath $werRoot) {
                $rootKey = Get-Item -LiteralPath $werRoot
                if ($rootKey.SubKeyCount -eq 0 -and $rootKey.ValueCount -eq 0) {
                    Remove-Item -LiteralPath $werRoot -Force
                    $werNotes += 'removed the LocalDumps parent created by this script because it was empty'
                }
                else { $werNotes += 'kept the LocalDumps parent because it was not empty' }
            }
        }
        catch {
            $werNotes += "failed to inspect/remove the created LocalDumps parent: $($_.Exception.Message)"
            $scriptExitCode = 1; $resultSummary = 'Windows Error Reporting cleanup failed'
        }
    }
    if ($artifactReady) {
        try {
            $werNotes | Set-Content -LiteralPath (Join-Path $artifactPath 'wer-localdumps.txt') -Encoding utf8; $resultPath = Join-Path $artifactPath 'result.txt'; $checksumPath = Join-Path $artifactPath 'checksums.sha256'
            @("exit_code=$scriptExitCode"; "result=$resultSummary"; "iterations_requested=$Iterations"; "iterations_completed=$completedIterations"; "failed_iteration=$failedIteration"; "finished_utc=$([DateTime]::UtcNow.ToString('o'))") | Set-Content -LiteralPath $resultPath -Encoding utf8
            Write-ArtifactChecksums -Root $artifactPath -OutputPath $checksumPath
        }
        catch { $scriptExitCode = 1; $resultSummary = "could not finalize artifacts: $($_.Exception.Message)"; try { Add-Content -LiteralPath $resultPath -Value "exit_code=$scriptExitCode`nresult=$resultSummary" -Encoding utf8 } catch { } }
    }
    if ($scriptExitCode -eq 0) { Write-Host -ForegroundColor Green "PASS: $resultSummary" } else { Write-Host -ForegroundColor Red "FAIL: $resultSummary" }; Write-Host "Artifacts: $artifactPath"
}
exit $scriptExitCode
