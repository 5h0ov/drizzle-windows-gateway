@echo off
setlocal
cd /d "%~dp0\.."

echo ========================================================
echo   Drizzle Gateway for Windows - 1-Click Native Updater
echo ========================================================
echo.

set VERSION=%1
if "%VERSION%"=="" (
    set VERSION=1.6.0
)

echo Updating / Building Drizzle Gateway Windows Native (v%VERSION%)...
echo.

where bun >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Bun is required to compile the backend.
    echo Please install Bun via: powershell -c "irm bun.sh/install.ps1 | iex"
    pause
    exit /b 1
)

where go >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Go is required to compile the native wrapper.
    echo Please install Go from https://go.dev/dl/
    pause
    exit /b 1
)

bun run scripts/update-gateway.mjs %VERSION%
if %ERRORLEVEL% neq 0 (
    echo.
    echo [ERROR] Build failed.
    pause
    exit /b %ERRORLEVEL%
)

echo.
echo ========================================================
echo   BUILD SUCCESSFUL!
echo   Run: DrizzleGateway.exe
echo ========================================================
echo.
pause