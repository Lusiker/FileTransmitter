# FileTransmitter

局域网文件传输应用，支持跨设备（PC、平板、手机）之间的大规模文件传输。

## 功能特性

- **跨平台支持**：PC、iPad、iPhone、Android 设备之间互传文件
- **大文件传输**：支持 GB 级别文件传输（分片上传 + 流式转发）
- **多文件批量传输**：支持同时传输数百个文件（解决移动端选择限制）
- **实时发现**：WebSocket 实时发现同一局域网内的其他设备
- **双向传输**：支持发送者和接收者两种角色
- **实时进度**：传输过程中实时显示进度
- **管理员监控**：设备连接、任务状态、存储使用实时监控
- **iOS/Safari 适配**：支持预览和逐文件下载
- **ZIP 打包下载**：支持批量下载所有文件为 ZIP

## 技术栈

### 后端
- Go 1.21+
- Gin (HTTP框架)
- Gorilla WebSocket (实时通信)
- Viper (配置管理)

### 前端
- Vue 3 + TypeScript
- Pinia (状态管理)
- Vue Router
- Element Plus (UI组件)
- Axios (HTTP客户端)
- Vite (构建工具)

## 项目结构

```
FileTransmitter/
├── cmd/server/main.go          # 程序入口
├── config.yaml                 # 配置文件
├── internal/
│   ├── config/                 # 配置加载
│   ├── handler/                # HTTP handlers
│   │   ├── device.go           # 设备管理 & WebSocket
│   │   ├── session.go          # 会话管理
│   │   ├── transfer.go         # 文件传输
│   │   └── admin.go            # 管理员监控 API
│   ├── model/                  # 数据模型
│   ├── service/                # 业务逻辑
│   │   ├── client_device.go    # 客户端设备管理
│   │   ├── session.go          # 会话服务
│   │   ├── transfer.go         # 传输服务
│   │   ├── relay.go            # 流式转发服务
│   │   ├── file_queue.go       # 滑动文件窗口
│   │   └── cleanup.go          # 清理服务
│   └── ws/                     # WebSocket
│       ├── hub.go              # 连接中心
│       ├── client.go           # 客户端连接
│       └── message.go          # 消息类型
├── web/                        # 前端
│   ├── src/
│   │   ├── views/              # 页面组件
│   │   │   ├── Dashboard.vue   # 首页
│   │   │   ├── Admin.vue       # 管理员监控页
│   │   │   ├── Sender/         # 发送者页面
│   │   │   └── Receiver/       # 接收者页面
│   │   ├── stores/             # Pinia stores
│   │   ├── composables/        # 组合式函数
│   │   ├── utils/              # 工具函数
│   │   │   └── platform.ts     # 平台检测
│   │   └── types/              # TypeScript 类型
│   └── vite.config.ts
└── README.md
```

## 快速开始

### 环境要求
- Go 1.21+
- Node.js 18+
- npm 或 yarn

### 安装依赖

```bash
# 后端依赖
go mod download

# 前端依赖
cd web && npm install
```

### 开发模式

```bash
# 启动后端 (端口 8080)
go run ./cmd/server/main.go

# 启动前端开发服务器 (端口 3000)
cd web && npm run dev -- --host
```

访问 `http://localhost:3000` 或 `http://<局域网IP>:3000`

### 生产构建

```bash
# 构建前端
cd web && npm run build

# 构建后端
go build -o server.exe ./cmd/server

# 运行（前端静态文件嵌入）
./server.exe
```

## 配置说明

`config.yaml` 配置文件：

```yaml
server:
  http_port: 8080        # HTTP 服务端口

device:
  id: ""                 # 设备ID（自动生成）
  name: ""               # 设备名称（自动生成）

transfer:
  chunk_size: 1048576    # 分块大小 (1MB)
  max_concurrent: 3      # 最大并发传输数
  temp_dir: "./tmp"      # 临时文件目录
```

## 使用说明

### 发送文件
1. 打开应用，设备名称默认随机生成
2. 选择要发送的文件（支持多次添加）
3. 从设备列表中选择接收者
4. 点击"发送请求"等待接收者确认

### 接收文件
1. 打开应用，选择"接收文件"角色
2. 等待传输请求出现
3. 点击"接收"确认
4. 文件传输完成后点击下载

### 管理员监控
- 访问 `/admin` 页面查看：
  - 设备连接状态
  - 任务进度
  - 临时存储占用
  - 支持清理已完成任务

### 移动端注意事项
- Android 浏览器可能限制单次文件选择数量，可使用"添加更多文件"按钮分批选择
- iOS/Safari 建议逐文件下载或使用 ZIP 批量下载

## API 接口

### 设备管理
- `GET /api/v1/devices` - 获取在线设备列表
- `GET /api/v1/ws` - WebSocket 连接

### 会话管理
- `POST /api/v1/sessions` - 创建传输会话
- `POST /api/v1/sessions/:id/accept` - 接受会话
- `POST /api/v1/sessions/:id/reject` - 拒绝会话

### 文件传输
- `POST /api/v1/transfer/upload` - 上传文件（小文件）
- `POST /api/v1/transfer/upload/chunk` - 分片上传（大文件）
- `GET /api/v1/transfer/:id/download/:file_id` - 下载文件
- `GET /api/v1/transfer/:id/download/zip` - ZIP打包下载

### 管理员接口
- `GET /api/v1/admin/status` - 系统状态概览
- `GET /api/v1/admin/devices` - 设备列表
- `GET /api/v1/admin/sessions` - 任务列表
- `GET /api/v1/admin/storage` - 存储使用情况
- `DELETE /api/v1/admin/storage/:session_id` - 清理指定会话
- `POST /api/v1/admin/storage/clean-completed` - 清理已完成任务

## WebSocket 消息类型

| 类型 | 说明 |
|------|------|
| `device_online` | 设备上线通知 |
| `device_offline` | 设备离线通知 |
| `session_created` | 新会话创建通知 |
| `session_accepted` | 会话被接受通知 |
| `transfer_progress` | 传输进度更新 |
| `file_complete` | 文件传输完成 |
| `chunk_data` | 分片数据（流式转发） |
| `chunk_ack` | 分片确认 |

## License

MIT