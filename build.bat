@echo off
setlocal enabledelayedexpansion

echo ============================================
echo    BTK Sorgu Go - Build Script
echo ============================================
echo.

:: Go path kontrolü - eğer PATH'te yoksa varsayılan konumu kullan
where go >nul 2>nul
if %errorlevel% neq 0 (
    if exist "C:\Program Files\Go\bin\go.exe" (
        set "PATH=C:\Program Files\Go\bin;%PATH%"
        echo [INFO] Go PATH'e eklendi: C:\Program Files\Go\bin
    ) else (
        echo [HATA] Go bulunamadi! Lutfen Go'yu yukleyin.
        exit /b 1
    )
)

:: Build klasörünü oluştur
if not exist "build" mkdir build

:: Versiyon bilgisi
set VERSION=1.0.0
set BUILD_TIME=%date% %time%

echo [INFO] Build klasoru: build\
echo [INFO] Versiyon: %VERSION%
echo.

:: Windows AMD64 Build
echo [1/4] Windows AMD64 build ediliyor...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o build\btk-sorgu-windows-amd64.exe main.go
if %errorlevel% neq 0 (
    echo [HATA] Windows AMD64 build basarisiz!
    exit /b 1
)
echo [OK] build\btk-sorgu-windows-amd64.exe

:: Windows ARM64 Build
echo [2/4] Windows ARM64 build ediliyor...
set GOOS=windows
set GOARCH=arm64
go build -ldflags="-s -w" -o build\btk-sorgu-windows-arm64.exe main.go
if %errorlevel% neq 0 (
    echo [HATA] Windows ARM64 build basarisiz!
    exit /b 1
)
echo [OK] build\btk-sorgu-windows-arm64.exe

:: Linux AMD64 Build
echo [3/4] Linux AMD64 build ediliyor...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o build\btk-sorgu-linux-amd64 main.go
if %errorlevel% neq 0 (
    echo [HATA] Linux AMD64 build basarisiz!
    exit /b 1
)
echo [OK] build\btk-sorgu-linux-amd64

:: Linux ARM64 Build
echo [4/4] Linux ARM64 build ediliyor...
set GOOS=linux
set GOARCH=arm64
go build -ldflags="-s -w" -o build\btk-sorgu-linux-arm64 main.go
if %errorlevel% neq 0 (
    echo [HATA] Linux ARM64 build basarisiz!
    exit /b 1
)
echo [OK] build\btk-sorgu-linux-arm64

echo.
echo ============================================
echo    Build Tamamlandi!
echo ============================================
echo.
echo Olusturulan dosyalar:
echo.
dir /b build\
echo.
echo Kullanim:
echo   Windows: build\btk-sorgu-windows-amd64.exe
echo   Linux:   ./build/btk-sorgu-linux-amd64
echo.

endlocal
