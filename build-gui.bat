@echo off
echo ============================================
echo   LuminaFlow GUI Build Script
echo ============================================
echo.
echo Starting build...

REM Set paths - prioritize mingw64 and go
set "PATH=C:\mingw64\bin;C:\go\bin;%PATH%"

REM Enable CGO
set CGO_ENABLED=1

REM Build GUI version
go build -tags gui -ldflags "-H windowsgui" -o LuminaFlow_gui.exe .

if %errorlevel% equ 0 (
    echo.
    echo ============================================
    echo   BUILD SUCCESSFUL!
    echo   Output: LuminaFlow_gui.exe
    echo ============================================
) else (
    echo.
    echo ============================================
    echo   BUILD FAILED!
    echo ============================================
)

pause
