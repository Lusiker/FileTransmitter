# FileTransmitter 流式转发架构升级方案

## 一、场景澄清（关键）

### 1.1 实际场景

> ❌ **不是**：单文件 100GB 极限问题  
> ✅ **而是**：多文件（200-300MB）累计 10GB～100GB

这两者在工程上是**完全不同难度等级**：

| 维度 | 单文件100GB | 多文件（200MB×N） |
|------|------------|-----------------|
| 浏览器内存压力 | ❌ 极高 | ✅ 可控 |
| IndexedDB | ❌ 不可能 | ⚠️ 可临时缓存 |
| 流式写入需求 | 必须 | 可弱化 |
| 失败恢复 | 极难 | ✅ 按文件恢复 |
| iOS 可行性 | ❌ | ⚠️ 勉强可做 |

**核心变化**：问题可以"离散化"为**文件级别**，而不是字节流级别。

---

## 二、当前系统架构

### 2.1 技术栈

**后端**：Go + Gin + Gorilla WebSocket + Viper  
**前端**：Vue 3 + TypeScript + Pinia + Element + Axios + Vite

### 2.2 核心流程（存储转发模式）

```
┌─────────────┐     ┌─────────────────────┐     ┌─────────────┐
│   发送端     │     │      后端服务器       │     │   接收端     │
│  (Sender)   │     │    (存储转发中心)     │     │  (Receiver) │
└─────────────┘     └─────────────────────┘     └─────────────┘
      │                      │                        │
      │  1. WebSocket 连接    │                        │
      │─────────────────────>│                        │
      │                      │<───────────────────────│
      │                      │   2. WebSocket 连接     │
      │                      │                        │
      │  3. 创建会话请求      │                        │
      │─────────────────────>│──广播──> session_created│
      │                      │                        │
      │                      │  4. 接收端确认/拒绝     │
      │                      │<───────────────────────│
      │<─广播─session_accepted│                        │
      │                      │                        │
      │  5. 分片上传 (5MB)    │                        │
      │─────────────────────>│                        │
      │                      │ 写入 ./tmp/{sessionID}/ │
      │                      │ ├─ chunks/{fileID}/    │
      │                      │ │  └─ chunk_0...N      │
      │                      │ └─ {fileID}.tmp        │
      │                      │                        │
      │<──进度广播──progress──│──广播──> progress      │
      │                      │                        │
      │  6. 合并分片          │                        │
      │                      │                        │
      │<──广播──complete─────│──广播──> complete      │
      │                      │                        │
      │                      │  7. HTTP Range 下载    │
      │                      │<───────────────────────│
      │                      │ http.ServeFile()       │
      │                      │────────────────────────>│
```

### 2.3 存储开销分析

| 累计大小 | 单文件大小 | 后端峰值占用 | 问题程度 |
|---------|-----------|-------------|---------|
| 10GB | 200MB×50个 | ~20GB | 中等 |
| 50GB | 300MB×170个 | ~100GB | 较严重 |
| 100GB | 200MB×500个 | ~200GB | 严重 |

**后端峰值 = 分片暂存 + 合并时临时文件 ≈ 2×累计大小**

### 2.4 当前方案优缺点

| 优点 | 缺点 |
|------|------|
| 接收端可随时下载 | 后端磁盘占用巨大 |
| 支持断点续传 | 大文件传输完成后清理不及时 |
| 支持多次下载 | 服务器磁盘成为瓶颈 |

---

## 三、推荐新架构：文件级任务队列 + 流式传输

### 3.1 架构升级

从：
```
大文件流
```

升级为：
```
Session
 ├── FileTask 1 (200MB)
 ├── FileTask 2 (300MB)
 ├── FileTask 3 (150MB)
 └── ...
```

### 3.2 每个文件独立处理

```
Sender → chunk stream → Backend → chunk stream → Receiver
         (ACK per file)    (内存缓冲)    (ACK per file)
```

**好处**：
- ✅ 每个文件可独立重试
- ✅ iOS 可以逐个下载
- ✅ 内存始终受控（≤ 单文件大小，约 200-300MB）

---

## 四、各平台能力评估

### 4.1 平台矩阵

| 平台 | 流式写入 | 后台传输 | 批量下载 | 推荐策略 |
|------|---------|---------|---------|---------|
| **Android** | ✅ 完全支持 | ✅ 可后台 | ✅ 并发 | WebSocket 流式转发 |
| **Chrome PC** | ✅ File System Access API | ⚠️ 需前台 | ✅ 可并发 | WebSocket + 流式写入 |
| **Safari PC** | ❌ 不支持 | ⚠️ 需前台 | ⚠️ 逐文件 | 逐文件 HTTP 下载 |
| **iOS Safari** | ❌ 硬限制 | ❌ 无后台 | ❌ 只能逐个 | **逐文件 HTTP 下载** |

### 4.2 iOS/iPadOS：唯一硬限制

**iOS 不能**：
- ❌ 后台连续下载 100GB
- ❌ 自动写入文件系统
- ❌ 无感接收
- ❌ File System Access API
- ❌ IndexedDB 存储大文件（限制 ~500MB-1GB）

**iOS 可以**：
- ✅ 前台逐文件下载
- ✅ 单文件 200-300MB 稳定下载
- ✅ 用户手动保存到 Files

**结论**：iOS 只能做"分文件下载"体验，这是 Apple 平台限制导致的必然结果。

---

## 五、推荐最终模式

### 5.1 模式选择策略

```
检测接收端平台：
├─ Android ──────────> WebSocket 流式转发（后端 relay）
├─ Chrome ───────────> WebSocket + File System Access API 流式写入
├─ Safari/iOS ───────> 逐文件 HTTP 下载（存储转发 + fallback）
└─ 接收端离线 ───────> 存储转发（等待接收端上线后下载）
```

### 5.2 模式详解

#### 🟢 模式1：WebSocket Relay（Android/Chrome）

```
Sender ──WebSocket──> Backend ──WebSocket──> Receiver
         分片上传              直接转发分片
                               File System Access API 写入
```

**后端存储**：几乎为 0（仅内存缓冲 ~10MB）

#### 🟡 模式2：逐文件 HTTP 下载（Safari/iOS）

```
Receiver:
for file in files:
    window.location = /download?file_id=xxx
    // 每个文件弹一次下载，用户保存到 Files
```

**用户体验**：
- 每个文件弹一次下载对话框
- 用户手动保存到 Files app
- 这是很多网盘的实际做法（可接受）

**后端存储**：需要存储转发（fallback）

#### 🔵 模式3：低存储优化（推荐）

```
后端角色：relay + 小缓存 + fallback 存储

流程：
1. 尝试流式转发（接收端在线）
2. 如果转发失败/接收端掉线 → fallback 到小缓存
3. 缓存满 → 等待接收端恢复或拒绝新传输
```

**不是"零存储"，而是"低存储"**：
- 缓存大小可配置（如 100MB buffer）
- 超出缓存的部分等待消费

---

## 六、协议设计变化

### 6.1 从全局 Chunk 改为文件级

**旧设计**：
```
全局 chunk index
```

**新设计**：
```
FileID + ChunkIndex
```

### 6.2 ACK 设计

```json
{
  "type": "chunk_ack",
  "file_id": "xxx",
  "chunk_index": 12,
  "status": "received"
}
```

### 6.3 失败恢复

```
文件失败 → 只重传这个文件
而不是 → 整个 100GB 重来
```

---

## 七、滑动文件窗口（关键优化）

### 7.1 概念

```
最多同时传 N 个文件（例如 2-3 个）
其他文件排队等待
```

### 7.2 好处

- ✅ 控制内存（最多缓存 2-3 个文件）
- ✅ 提升吞吐（并发传输）
- ✅ 避免 Safari 崩（单个文件不会太大）
- ✅ 失败隔离（一个文件失败不影响其他）

### 7.3 实现

```go
// 后端维护传输队列
type TransferQueue struct {
    activeFiles   map[string]*FileTransfer  // 正在传输的文件（最多N个）
    pendingFiles  []*FileTransfer           // 等待队列
    completedFiles []*FileTransfer          // 已完成
    maxConcurrent int                       // 最大并发数（如3）
}
```

---

## 八、核心问题解决方案

### 8.1 问题：如何在 Safari/iOS 上实现大文件传输？

**答案**：逐文件 HTTP 下载

```ts
// 前端实现
for (const file of session.files) {
  const downloadUrl = `/api/v1/transfer/${sessionId}/download/${file.id}`
  window.location.href = downloadUrl
  // 等待用户保存后继续下一个
}
```

### 8.2 问题：如何保证 WebSocket 分片传输可靠性？

**答案**：ACK + 重传机制

```
Sender 发送 chunk → Backend 转发 → Receiver 收到
                                    ↓
                              发送 ACK 给 Backend
                                    ↓
                              Backend 通知 Sender
                                    ↓
                              Sender 发送下一个 chunk
```

### 8.3 问题：如何处理接收端中途断开？

**答案**：Fallback 到存储转发

```
if (receiver.disconnected) {
    // 停止流式转发
    // 将当前 chunk 缓存到临时存储
    // 等待接收端恢复后继续
}
```

### 8.4 问题：用户体验差异如何处理？

**答案**：接受现实，透明说明

| 平台 | 体验 | 说明 |
|------|------|------|
| Android/Chrome | 无感流式 | 自动接收，自动保存 |
| Safari/iOS | 手动下载 | 需逐个点击下载保存 |

**这是 Apple 平台限制导致的必然结果，无法绕过。**

---

## 九、后端架构改造

### 9.1 后端角色变化

从：
```
存储中心
```

变为：
```
Relay + 小缓存 + Fallback 存储
```

### 9.2 核心组件

```go
type TransferEngine struct {
    // 流式转发通道
    relayChannels map[string]*RelayChannel  // sessionID -> channel
    
    // 小缓存（内存缓冲区）
    bufferPool    *BufferPool               // 最大100MB
    
    // Fallback 存储（当接收端离线时）
    fallbackStore *FallbackStore            // 临时存储
    
    // 平台检测
    platformDetector *PlatformDetector
}

type RelayChannel struct {
    senderConn   *WebSocketConn
    receiverConn *WebSocketConn
    activeFiles  map[string]*FileStream
    buffer       []byte  // 当前文件的内存缓冲
}
```

### 9.3 流程

```go
func (e *TransferEngine) HandleChunk(sessionID, fileID, chunkIndex int, data []byte) {
    // 1. 检测接收端状态
    if e.isReceiverOnline(sessionID) && e.canStream(sessionID) {
        // 流式转发
        e.relayToReceiver(sessionID, fileID, chunkIndex, data)
    } else {
        // Fallback 到缓存/存储
        e.bufferOrStore(sessionID, fileID, chunkIndex, data)
    }
}
```

---

## 十、前端三端统一策略

### 10.1 Chrome/Android

```ts
// 使用 File System Access API 流式写入
async function streamReceive(sessionId: string, files: FileInfo[]) {
  for (const file of files) {
    const handle = await window.showSaveFilePicker({
      suggestedName: file.name
    })
    const writable = await handle.createWritable()
    
    // 通过 WebSocket 接收分片并写入
    const stream = new WebSocketStream(sessionId, file.id)
    await stream.pipeTo(writable)
  }
}
```

### 10.2 Safari/iOS

```ts
// 逐文件 HTTP 下载
function downloadFilesSequentially(sessionId: string, files: FileInfo[]) {
  let index = 0
  
  function downloadNext() {
    if (index >= files.length) return
    
    const file = files[index]
    const url = `/api/v1/transfer/${sessionId}/download/${file.id}`
    
    // 触发下载
    window.location.href = url
    
    // 用户保存后点击"继续下一个"
    index++
  }
  
  downloadNext()
}
```

### 10.3 统一入口

```ts
function receiveFiles(sessionId: string, files: FileInfo[]) {
  const platform = detectPlatform()
  
  switch (platform) {
    case 'android':
    case 'chrome':
      streamReceive(sessionId, files)
      break
    case 'safari':
    case 'ios':
      downloadFilesSequentially(sessionId, files)
      break
  }
}
```

---

## 十一、当前代码关键位置

| 模块 | 文件路径 | 功能 |
|------|---------|------|
| **分片上传** | `internal/service/transfer.go:136-217` | UploadFileChunk |
| **分片合并** | `internal/service/transfer.go:220-284` | mergeChunks |
| **下载处理** | `internal/service/transfer.go:306-339` | DownloadFile |
| **WebSocket Hub** | `internal/ws/hub.go` | 连接管理中心 |
| **WebSocket Client** | `internal/ws/client.go` | 单个连接处理 |
| **消息类型** | `internal/ws/message.go` | 定义消息结构 |
| **前端上传** | `web/src/composables/useFileTransfer.ts:125-166` | uploadFileChunked |
| **前端下载** | `web/src/composables/useFileTransfer.ts:49-68` | handleDownloadFile |
| **设备状态** | `web/src/stores/device.ts` | WebSocket 连接管理 |
| **会话状态** | `web/src/stores/session.ts` | 传输进度管理 |

---

## 十二、实施路线图

### Phase 1：协议升级（优先）

1. 定义文件级传输协议（FileID + ChunkIndex + ACK）
2. 实现滑动文件窗口
3. 后端支持 relay 模式

### Phase 2：平台适配

1. Chrome/Android：File System Access API 流式写入
2. Safari/iOS：逐文件 HTTP 下载 UI

### Phase 3：优化

1. 低存储 fallback 机制
2. 断点续传（文件级）
3. 传输队列管理

---

## 十三、最终结论

> ✅ **问题已从"不可解"降级为"工程可实现"**

关键点：
1. 文件级任务队列代替字节流
2. Android/Chrome：流式转发（后端零存储）
3. Safari/iOS：逐文件下载（后端低存储 fallback）
4. 接受 iOS 体验割裂（Apple 平台限制）

**下一步**：设计文件级传输协议（带 ACK + 滑窗）