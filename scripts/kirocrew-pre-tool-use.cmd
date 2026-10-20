@echo off
rem Copyright (c) 2026 Tiago Vilas Boas.
rem SPDX-License-Identifier: Apache-2.0
rem Downshift by Tiago Vilas Boas
rem https://github.com/tiagovilasboas/downshift
rem
rem Windows sibling of kirocrew-pre-tool-use.sh. KiroCrew wants an absolute
rem executable with no arguments; this forwards stdin to `downshift kirocrew`
rem and returns its exit code (0 allow / 2 block). Register this .cmd on Windows
rem instead of the .sh. The router binary sits one level up from this script.
setlocal
set "ROUTER=%~dp0..\downshift.exe"
if not exist "%ROUTER%" set "ROUTER=downshift"
"%ROUTER%" kirocrew
exit /b %ERRORLEVEL%
