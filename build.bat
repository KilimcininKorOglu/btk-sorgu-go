@echo off
setlocal enabledelayedexpansion

set BINARY_NAME=btk-sorgu
set BUILD_DIR=bin

:: Get version info
for /f "tokens=*" %%i in ('git describe --tags --always --dirty 2^>nul') do set VERSION=%%i
if not defined VERSION set VERSION=dev

for /f "tokens=*" %%i in ('git rev-parse --short HEAD 2^>nul') do set COMMIT=%%i
if not defined COMMIT set COMMIT=unknown

for /f "tokens=*" %%i in ('powershell -command "Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ' -AsUTC"') do set BUILD_DATE=%%i
if not defined BUILD_DATE set BUILD_DATE=unknown

set LDFLAGS=-s -w -X 'main.version=%VERSION%' -X 'main.commit=%COMMIT%' -X 'main.buildDate=%BUILD_DATE%'

if "%~1"=="" goto help
goto %~1

:build
    if not exist %BUILD_DIR% mkdir %BUILD_DIR%
    go build -ldflags "%LDFLAGS%" -o %BUILD_DIR%\%BINARY_NAME%.exe .
    goto end

:clean
    if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
    go clean
    goto end

:test
    go test ./...
    goto end

:test-race
    go test -race ./...
    goto end

:test-cover
    go test -cover ./...
    goto end

:test-verbose
    go test -v ./...
    goto end

:bench
    go test -bench=. -benchmem ./...
    goto end

:run
    call :build
    %BUILD_DIR%\%BINARY_NAME%.exe
    goto end

:fmt
    go fmt ./...
    goto end

:vet
    go vet ./...
    goto end

:lint
    call :fmt
    call :vet
    goto end

:help
    echo Available commands:
    echo   build        - Build the binary to bin\
    echo   clean        - Remove build artifacts
    echo   test         - Run all tests
    echo   test-race    - Run tests with race detector
    echo   test-cover   - Run tests with coverage
    echo   test-verbose - Run tests with verbose output
    echo   bench        - Run benchmarks
    echo   run          - Build and run the server
    echo   fmt          - Format code
    echo   vet          - Run go vet
    echo   lint         - Run fmt and vet
    goto end

:end
    endlocal
