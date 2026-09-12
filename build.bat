@echo off
setlocal
cd /d "%~dp0"

title Drizzle Gateway - Builder
echo ===================================================
echo   Building Drizzle Gateway Windows Native Wrapper
echo ===================================================
echo.

:: Ensure local Go toolchain is used
set GOTOOLCHAIN=local

:: Check if Go is installed
where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Go compiler was not found in your system PATH.
    echo Please install Go from https://go.dev/dl/ and restart.
    echo.
    pause
    exit /b 1
)

echo [1/2] Compiling native Windows binary...
go build -ldflags="-H windowsgui -s -w" -o DrizzleGateway.exe .
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Build failed! Please inspect the error messages above.
    pause
    exit /b %ERRORLEVEL%
)

echo [2/2] Finalizing build...
echo.
echo ===================================================
echo   [SUCCESS] DrizzleGateway.exe is ready!
echo ===================================================
echo.
powershell -NoProfile -Command "$bytes = (Get-Item DrizzleGateway.exe).Length; $mb = [math]::Round($bytes / 1MB, 2); Write-Host "Executable Size: $mb MB""
echo.
echo You can now double-click DrizzleGateway.exe to launch.
echo.
pause
