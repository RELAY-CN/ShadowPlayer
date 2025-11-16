@echo off
chcp 65001 >nul
setlocal EnableDelayedExpansion

echo ========================================
echo    ShadowPlayer 多平台编译脚本
echo ========================================

:: 清除PATH中的问题条目（临时）
set "PATH="
for %%A in ("%ORIGINAL_PATH:;=";"%") do (
    if /i not "%%~A"=="*VMware*" (
        if defined PATH (
            set "PATH=!PATH!;%%~A"
        ) else (
            set "PATH=%%~A"
        )
    )
)

set "GO_ROOT=R:\Single Program Files\go"
set "GOROOT=%GO_ROOT%"
set "PATH=%GO_ROOT%\bin;%PATH%"

:: 验证路径是否存在
if not exist "%GO_ROOT%" (
    echo [错误] Go根目录不存在: "%GO_ROOT%"
    pause
    exit /b 1
)

if not exist "%GO_ROOT%\bin\go.exe" (
    echo [错误] 在路径中未找到go.exe: "%GO_ROOT%\bin\go.exe"
    pause
    exit /b 1
)

:: 验证Go安装
echo 检查Go环境...
call "%GO_ROOT%\bin\go.exe" version >nul 2>&1
if errorlevel 1 (
    echo [错误] Go编译器执行失败，请检查安装: "%GO_ROOT%"
    echo 当前PATH: %PATH%
    pause
    exit /b 1
)

echo Go路径设置成功: "%GOROOT%"
"%GO_ROOT%\bin\go.exe" version
echo.

:: 设置变量
set "SOURCE_FILE=D:\home\RELAY-CN_Group\ShadowPlayer\src\main.go"
set "OUTPUT_DIR=D:\home\RELAY-CN_Group\ShadowPlayer\build"
set "WORK_DIR=D:\home\RELAY-CN_Group\ShadowPlayer"
set "BUILD_FLAGS=-trimpath -ldflags="-s -w""

:: 创建输出目录
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

:: 目标平台配置
set PLATFORMS[0].os=windows
set PLATFORMS[0].arch=amd64
set PLATFORMS[0].ext=.exe

set PLATFORMS[1].os=windows
set PLATFORMS[1].arch=386
set PLATFORMS[1].ext=.exe

set PLATFORMS[2].os=linux
set PLATFORMS[2].arch=amd64
set PLATFORMS[2].ext=

set PLATFORMS[3].os=linux
set PLATFORMS[3].arch=arm64
set PLATFORMS[3].ext=

set PLATFORMS[4].os=darwin
set PLATFORMS[4].arch=amd64
set PLATFORMS[4].ext=

set PLATFORMS[5].os=darwin
set PLATFORMS[5].arch=arm64
set PLATFORMS[5].ext=

:: 编译计数器
set /a SUCCESS_COUNT=0
set /a FAIL_COUNT=0

echo 开始多平台编译...
echo 源文件: %SOURCE_FILE%
echo 输出目录: %OUTPUT_DIR%
echo.

:: 遍历所有平台进行编译
for /l %%i in (0,1,5) do (
    set "CURRENT_OS=!PLATFORMS[%%i].os!"
    set "CURRENT_ARCH=!PLATFORMS[%%i].arch!"
    set "CURRENT_EXT=!PLATFORMS[%%i].ext!"
    
    if defined CURRENT_OS (
        set "OUTPUT_NAME=ShadowPlayer_!CURRENT_OS!_!CURRENT_ARCH!!CURRENT_EXT!"
        set "OUTPUT_PATH=%OUTPUT_DIR%\!OUTPUT_NAME!"
        
        echo [编译中] !CURRENT_OS!-!CURRENT_ARCH!...
        
        :: 设置环境变量并编译
        set GOOS=!CURRENT_OS!
        set GOARCH=!CURRENT_ARCH!
        
        cd /d "%WORK_DIR%"
        go build %BUILD_FLAGS% -o "!OUTPUT_PATH!" "%SOURCE_FILE%"
        
        if !errorlevel! equ 0 (
            echo [成功] !OUTPUT_NAME!
            set /a SUCCESS_COUNT+=1
            
            :: 如果是Windows可执行文件，创建配套的批处理文件
            if "!CURRENT_OS!"=="windows" (
                echo @echo off > "%OUTPUT_DIR%\运行_!CURRENT_OS!_!CURRENT_ARCH!.bat"
                echo chcp 65001 >nul >> "%OUTPUT_DIR%\运行_!CURRENT_OS!_!CURRENT_ARCH!.bat"
                echo echo 正在启动 ShadowPlayer... >> "%OUTPUT_DIR%\运行_!CURRENT_OS!_!CURRENT_ARCH!.bat"
                echo echo ======================================== >> "%OUTPUT_DIR%\运行_!CURRENT_OS!_!CURRENT_ARCH!.bat"
                echo "!OUTPUT_NAME!" >> "%OUTPUT_DIR%\运行_!CURRENT_OS!_!CURRENT_ARCH!.bat"
                echo pause >> "%OUTPUT_DIR%\运行_!CURRENT_OS!_!CURRENT_ARCH!.bat"
            )
        ) else (
            echo [失败] !CURRENT_OS!-!CURRENT_ARCH!
            set /a FAIL_COUNT+=1
        )
        echo.
    )
)

:: 显示编译结果
echo ========================================
echo 编译完成!
echo 成功: %SUCCESS_COUNT% 个平台
echo 失败: %FAIL_COUNT% 个平台
echo.

:: 显示生成的文件
echo 生成的文件:
dir "%OUTPUT_DIR%\ShadowPlayer_*" /b

echo.
echo 输出目录: %OUTPUT_DIR%
echo ========================================

:: 如果是在Windows上编译，询问是否运行Windows版本
if %FAIL_COUNT% equ 0 (
    echo 是否立即运行Windows版本? (Y/N)
    set /p RUN_CHOICE=
    if /i "!RUN_CHOICE!"=="Y" (
        if exist "%OUTPUT_DIR%\ShadowPlayer_windows_amd64.exe" (
            cd /d "%OUTPUT_DIR%"
            echo 正在启动 ShadowPlayer Windows版本...
            echo ========================================
            ShadowPlayer_windows_amd64.exe
        )
    )
)

endlocal
pause