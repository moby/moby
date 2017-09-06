@echo off

setlocal

set _EXITCODE=0

set _DEPSFILE=%TEMP%\consul-deps.txt
go list -f "{{range .TestImports}}{{.}} {{end}}" .\... >%_DEPSFILE%

set _PKGSFILE=%TEMP%\consul-pkgs.txt
go list .\... >%_PKGSFILE%

set _VETARGS=-asmdecl -atomic -bool -buildtags -copylocks -methods^
 -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
if defined VETARGS set _VETARGS=%VETARGS%

:deps
echo --^> Installing build dependencies
for /f "delims=" %%d in (%_DEPSFILE%) do go get -d -v .\... %%d

if [%1]==[] goto all
if x%1==xdeps goto end
goto args

:args
for %%a in (all,cover,integ,test,vet,updatedeps) do (if x%1==x%%a goto %%a)
echo.
echo Unknown make target: %1
echo Expected one of "all", "cover", "deps", "integ", "test", "vet", or "updatedeps".
set _EXITCODE=1
goto end

:all
md bin 2>NUL
call .\scripts\windows\build.bat %CD%
if not errorlevel 1 goto end
echo.
echo BUILD FAILED
set _EXITCODE=%ERRORLEVEL%
goto end

:cover
set _COVER=--cover
go tool cover 2>NUL
if %ERRORLEVEL% EQU 3 go get golang.org/x/tools/cmd/cover
goto test

:integ
set INTEG_TESTS=yes
goto test

:test
call .\scripts\windows\verify_no_uuid.bat %CD%
if %ERRORLEVEL% EQU 0 goto _test
echo.
echo UUID verification failed.
set _EXITCODE=%ERRORLEVEL%
goto end
:_test
for /f "delims=" %%p in (%_PKGSFILE%) do (
	go test %_COVER% %%p
	if errorlevel 1 set _TESTFAIL=1
)
if x%_TESTFAIL%==x1 set _EXITCODE=1 && goto end
goto vet

:vet
go tool vet 2>NUL
if %ERRORLEVEL% EQU 3 go get golang.org/x/tools/cmd/vet
echo --^> Running go tool vet %_VETARGS%
go tool vet %_VETARGS% .
echo.
if %ERRORLEVEL% EQU 0 echo ALL TESTS PASSED && goto end
echo Vet found suspicious constructs. Please check the reported constructs
echo and fix them if necessary before submitting the code for reviewal.
set _EXITCODE=%ERRORLEVEL%
goto end

:updatedeps
echo --^> Updating build dependencies
for /f "delims=" %%d in (%_DEPSFILE%) do go get -d -f -u .\... %%d
goto end

:end
del /F %_DEPSFILE% %_PKGSFILE% 2>NUL
exit /B %_EXITCODE%
