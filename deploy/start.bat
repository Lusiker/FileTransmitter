@echo off
title FileTransmitter

echo ========================================
echo   FileTransmitter - 局域网文件传输
echo ========================================
echo.

cd /d "%~dp0"

:: 获取本机IP
for /f "tokens=2 delims=:" %%a in ('ipconfig ^| findstr /c:"IPv4"') do (
    set LOCAL_IP=%%a
    goto :show_ip
)
:show_ip
set LOCAL_IP=%LOCAL_IP: =%

echo 服务启动中...
echo.
echo 访问地址:
echo   本机: http://localhost:8080
echo   局域网: http://%LOCAL_IP%:8080
echo.
echo ----------------------------------------
echo   关闭此窗口将停止所有服务
echo ----------------------------------------
echo.

:: 启动后端服务（前台运行）
server.exe

:: 如果server.exe退出
echo.
echo 服务已停止
pause