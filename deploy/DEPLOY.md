# FileTransmitter 部署手册

## 目录结构

```
deploy/
├── server.exe       # 后端可执行文件
├── start.bat        # Windows启动脚本
├── config.yaml      # 配置文件
└── web/             # 前端静态文件
    └── dist/
        ├── index.html
        ├── assets/
        └── favicon.svg
```

## 系统要求

- **操作系统**: Windows 10/11, Linux, macOS
- **网络**: 局域网环境，所有设备需在同一网段
- **端口**: 8080 (HTTP)

## 快速部署

### Windows（推荐）

1. 将 `deploy` 目录复制到目标服务器
2. 双击运行 `start.bat`
3. 看到启动信息后，访问显示的地址
4. 关闭窗口即可停止服务

### Linux/macOS

1. 重新编译 Linux/macOS 版本：
   ```bash
   # Linux
   GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o server ./cmd/server

   # macOS
   GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o server ./cmd/server
   ```
2. 运行：
   ```bash
   chmod +x server
   ./server
   ```

## 配置说明

`config.yaml`:

```yaml
server:
  http_port: 8080        # HTTP 服务端口

device:
  id: ""                 # 设备ID（自动生成，部署时留空）
  name: ""               # 设备名称（自动生成，部署时留空）
  role: "sender"         # 默认角色

transfer:
  chunk_size: 5242880    # 分片大小 (5MB)
  max_concurrent: 3      # 最大并发传输数
  temp_dir: "./tmp"      # 临时文件目录

log:
  level: "info"          # 日志级别
```

**生产环境建议**:
- `temp_dir` 设置为绝对路径，如 `/var/filetransmitter/tmp`
- 确保临时目录有足够磁盘空间

## 防火墙配置

### Windows

```powershell
# 添加入站规则（管理员权限）
netsh advfirewall firewall add rule name="FileTransmitter 8080" dir=in action=allow protocol=tcp localport=8080
```

### Linux (iptables)

```bash
# 开放端口
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
```

### Linux (firewalld)

```bash
firewall-cmd --add-port=8080/tcp --permanent
firewall-cmd --reload
```

## 使用方式

1. **发送端**:
   - 访问 `http://<服务器IP>:8080`
   - 点击"发送文件"
   - 选择文件和接收者
   - 点击"发送请求"

2. **接收端**:
   - 访问 `http://<服务器IP>:8080`
   - 点击"接收文件"
   - 等待传输请求
   - 点击"接收"确认

3. **管理员监控**:
   - 访问 `http://<服务器IP>:8080/admin`
   - 查看设备连接、任务状态、存储使用

## 常见问题

### Q: 手机无法访问？

1. 确认手机与服务器在同一局域网
2. 检查服务器防火墙是否开放 8080 端口
3. 清除手机浏览器缓存

### Q: 传输失败？

1. 检查临时目录是否有足够空间
2. 查看管理员页面的存储占用
3. 清理已完成的任务释放空间

### Q: WebSocket 连接断开？

1. 检查网络稳定性
2. 确保没有代理服务器干扰 WebSocket
3. 页面会自动重连，等待几秒即可

## 后台运行

### Windows (使用任务计划程序)

1. 打开"任务计划程序"
2. 创建基本任务
3. 设置触发器为"计算机启动时"
4. 操作：启动程序 `server.exe`
5. 起始于：deploy 目录路径

### Linux (systemd)

创建服务文件 `/etc/systemd/system/filetransmitter.service`:

```ini
[Unit]
Description=FileTransmitter Service
After=network.target

[Service]
Type=simple
User=filetransmitter
WorkingDirectory=/opt/filetransmitter
ExecStart=/opt/filetransmitter/server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启用服务：
```bash
systemctl daemon-reload
systemctl enable filetransmitter
systemctl start filetransmitter
```

## 停止服务

- **Windows**: 关闭命令行窗口或任务管理器结束进程
- **Linux**: `systemctl stop filetransmitter` 或 `kill <PID>`