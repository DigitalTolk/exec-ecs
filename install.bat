@echo off
setlocal enabledelayedexpansion

:: Check for Administrator privileges
:: This will fail if the user is not running as Administrator
whoami /groups | find "S-1-5-32-544" >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] This script must be run as Administrator.
    echo Please right-click this file and select "Run as Administrator."
    pause
    exit /b 1
)

:: Configuration
set REPO=DigitalTolk/exec-ecs

:: Check if curl exists
where curl >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] curl is not installed. Please install curl first.
    pause
    exit /b 1
)

:: Get the latest or specified version
set "VERSION="
if "%~1"=="" (
    echo Fetching latest release version...
    for /f "tokens=2 delims=:, " %%A in ('curl -s https://api.github.com/repos/%REPO%/releases/latest ^| findstr /i "tag_name"') do set "VERSION=%%A"
    set "VERSION=!VERSION:~1,-1!"
) else (
    set "VERSION=v%~1"
)

if not defined VERSION (
    echo [ERROR] Failed to fetch the latest version.
    pause
    exit /b 1
)

echo Version: %VERSION%

:: Construct the filename and download URL
set "FILENAME=exec-ecs_Windows_x86_64.zip"
set "URL=https://github.com/%REPO%/releases/download/%VERSION%/%FILENAME%"

:: Create a temporary directory
set "TEMP_DIR=%TEMP%\exec-ecs"
if exist "%TEMP_DIR%" rd /s /q "%TEMP_DIR%"
mkdir "%TEMP_DIR%"

:: Download the file
echo Downloading %FILENAME% from %URL%...
curl -Lo "%TEMP_DIR%\%FILENAME%" %URL%
if not exist "%TEMP_DIR%\%FILENAME%" (
    echo [ERROR] Download failed.
    pause
    exit /b 1
)

:: Extract the archive
echo Extracting %FILENAME%...
powershell -command "Expand-Archive -Path '%TEMP_DIR%\%FILENAME%' -DestinationPath '%TEMP_DIR%'"
if not exist "%TEMP_DIR%\exec-ecs.exe" (
    echo [ERROR] Extraction failed.
    pause
    exit /b 1
)

:: Move the binary to a directory in PATH
echo Moving exec-ecs.exe to %ProgramFiles%\exec-ecs...
if not exist "%ProgramFiles%\exec-ecs" mkdir "%ProgramFiles%\exec-ecs"
move /y "%TEMP_DIR%\exec-ecs.exe" "%ProgramFiles%\exec-ecs\"

:: Add to PATH
echo Adding exec-ecs to PATH...
setx PATH "%PATH%;%ProgramFiles%\exec-ecs"

:: Verify installation
"%ProgramFiles%\exec-ecs\exec-ecs.exe" --version
if %errorlevel% equ 0 (
    echo Installation completed successfully!
) else (
    echo [ERROR] Installation verification failed.
    pause
    exit /b 1
)

pause
exit /b 0
