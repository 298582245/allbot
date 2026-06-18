@echo off
chcp 65001 >nul
set VERSION=v1.0.1
set APP_NAME=allbot
set CHECKSUM_FILE=checksums-%VERSION%.txt

echo ========================================
echo  构建 %APP_NAME% %VERSION%
echo ========================================
echo.

echo [1/5] 构建 Linux AMD64...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o %APP_NAME%-%VERSION%-linux-amd64
if %errorlevel% neq 0 (
    echo ❌ Linux AMD64 构建失败！
) else (
    echo ✅ Linux AMD64 构建成功
)
echo.

echo [2/5] 构建 Linux ARM64...
set GOOS=linux
set GOARCH=arm64
go build -ldflags="-s -w" -o %APP_NAME%-%VERSION%-linux-arm64
if %errorlevel% neq 0 (
    echo ❌ Linux ARM64 构建失败！
) else (
    echo ✅ Linux ARM64 构建成功
)
echo.

echo [3/5] 构建 Windows AMD64 (64位)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o %APP_NAME%-%VERSION%-windows-amd64.exe
if %errorlevel% neq 0 (
    echo ❌ Windows AMD64 构建失败！
) else (
    echo ✅ Windows AMD64 构建成功
)
echo.

echo [4/5] 构建 Windows 386 (32位)...
set GOOS=windows
set GOARCH=386
go build -ldflags="-s -w" -o %APP_NAME%-%VERSION%-windows-386.exe
if %errorlevel% neq 0 (
    echo ❌ Windows 386 构建失败！
) else (
    echo ✅ Windows 386 构建成功
)
echo.

echo [5/5] 生成 checksums 文件...
if exist %CHECKSUM_FILE% del %CHECKSUM_FILE%

echo 生成 SHA256 校验和...
(for %%f in (%APP_NAME%-%VERSION%-*) do (
    if exist "%%f" (
        certutil -hashfile "%%f" SHA256 | find /v "SHA256" | find /v "CertUtil"
    )
)) > %CHECKSUM_FILE%

if exist %CHECKSUM_FILE% (
    echo ✅ checksums 文件生成成功
) else (
    echo ⚠️ 没有成功构建的文件，checksums 文件未生成
)
echo.

echo 生成的文件:
dir %APP_NAME%-%VERSION%-* 2>nul
echo.
pause