// 流式接收逻辑（Chrome/Android）
// 使用 File System Access API 直接写入本地文件

import { ref, computed } from 'vue'
import type { FileInfo } from '@/types'
import { useDeviceStore } from '@/stores/device'
import { useSessionStore } from '@/stores/session'
import { supportsFileSystemAccess, detectPlatform } from '@/utils/platform'

// 分片大小：5MB（与后端一致）
const CHUNK_SIZE = 5 * 1024 * 1024

// 当前接收状态
const receivingFiles = ref<Map<string, number>>(new Map()) // fileID -> received chunks
const fileHandles = ref<Map<string, FileSystemFileHandle>>(new Map())
const writableStreams = ref<Map<string, FileSystemWritableFileStream>>(new Map())

export function useStreamReceiver() {
  const deviceStore = useDeviceStore()
  const sessionStore = useSessionStore()

  const isReceiving = computed(() => receivingFiles.value.size > 0)

  /**
   * 开始接收文件
   * 使用 File System Access API 选择保存位置
   */
  async function startReceiving(sessionId: string, file: FileInfo): Promise<boolean> {
    // 检查是否支持 File System Access API
    if (!supportsFileSystemAccess()) {
      console.warn('[StreamReceiver] File System Access API not supported')
      return false
    }

    try {
      // 让用户选择保存位置
      const handle = await window.showSaveFilePicker({
        suggestedName: file.name,
        types: [
          {
            description: 'File',
            accept: {
              '*/*': [getExtension(file.name)],
            },
          },
        ],
      })

      // 创建可写流
      const writable = await handle.createWritable()

      // 保存 handle 和 writable
      fileHandles.value.set(file.id, handle)
      writableStreams.value.set(file.id, writable)
      receivingFiles.value.set(file.id, 0)

      // 通知后端准备好接收
      notifyReceiverReady(sessionId, true)

      console.log('[StreamReceiver] Ready to receive:', file.name)
      return true
    } catch (error) {
      console.error('[StreamReceiver] Failed to start receiving:', error)
      return false
    }
  }

  /**
   * 处理接收到的分片数据
   */
  async function handleChunkData(data: {
    session_id: string
    file_id: string
    chunk_index: number
    total_chunks: number
    data: string // base64 encoded
    size: number
  }): Promise<void> {
    const writable = writableStreams.value.get(data.file_id)
    if (!writable) {
      console.warn('[StreamReceiver] No writable stream for file:', data.file_id)
      return
    }

    try {
      // 解码 base64 数据
      const binaryData = base64ToArrayBuffer(data.data)

      // 写入分片
      const offset = data.chunk_index * CHUNK_SIZE
      await writable.seek(offset)
      await writable.write(binaryData)

      // 更新接收进度
      receivingFiles.value.set(data.file_id, data.chunk_index + 1)

      // 发送 ACK 给后端
      sendChunkAck(data.session_id, data.file_id, data.chunk_index, 'ok')

      // 更新前端进度显示
      sessionStore.updateProgress({
        session_id: data.session_id,
        file_id: data.file_id,
        bytes: (data.chunk_index + 1) * CHUNK_SIZE,
        percent: Math.round(((data.chunk_index + 1) / data.total_chunks) * 100),
        phase: 'uploading',
      })

      console.log('[StreamReceiver] Chunk received:', data.chunk_index + 1, '/', data.total_chunks)
    } catch (error) {
      console.error('[StreamReceiver] Failed to write chunk:', error)
      sendChunkAck(data.session_id, data.file_id, data.chunk_index, 'error')
    }
  }

  /**
   * 完成文件接收
   */
  async function finishReceiving(fileId: string): Promise<void> {
    const writable = writableStreams.value.get(fileId)
    if (writable) {
      try {
        await writable.close()
        console.log('[StreamReceiver] File saved:', fileId)
      } catch (error) {
        console.error('[StreamReceiver] Failed to close writable:', error)
      }
    }

    // 清理
    writableStreams.value.delete(fileId)
    fileHandles.value.delete(fileId)
    receivingFiles.value.delete(fileId)
  }

  /**
   * 发送分片 ACK
   */
  function sendChunkAck(sessionId: string, fileId: string, chunkIndex: number, status: string) {
    const ws = deviceStore.getWebSocketConnection()
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      console.warn('[StreamReceiver] WebSocket not connected')
      return
    }

    const message = {
      type: 'chunk_ack',
      data: {
        session_id: sessionId,
        file_id: fileId,
        chunk_index: chunkIndex,
        status: status,
      },
    }

    ws.send(JSON.stringify(message))
  }

  /**
   * 通知后端接收端准备好
   */
  function notifyReceiverReady(sessionId: string, supportsStreaming: boolean) {
    const ws = deviceStore.getWebSocketConnection()
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      console.warn('[StreamReceiver] WebSocket not connected')
      return
    }

    const message = {
      type: 'receiver_ready',
      data: {
        session_id: sessionId,
        platform: detectPlatform(),
        supports_streaming: supportsStreaming,
      },
    }

    ws.send(JSON.stringify(message))
  }

  /**
   * 取消接收
   */
  async function cancelReceiving(fileId: string): Promise<void> {
    const writable = writableStreams.value.get(fileId)
    if (writable) {
      try {
        await writable.abort()
      } catch (error) {
        console.error('[StreamReceiver] Failed to abort writable:', error)
      }
    }

    // 清理
    writableStreams.value.delete(fileId)
    fileHandles.value.delete(fileId)
    receivingFiles.value.delete(fileId)
  }

  /**
   * 获取接收进度
   */
  function getReceivingProgress(fileId: string): number {
    return receivingFiles.value.get(fileId) || 0
  }

  return {
    isReceiving,
    startReceiving,
    handleChunkData,
    finishReceiving,
    cancelReceiving,
    getReceivingProgress,
    sendChunkAck,
    notifyReceiverReady,
  }
}

// 辅助函数

/**
 * 获取文件扩展名
 */
function getExtension(filename: string): string {
  const parts = filename.split('.')
  if (parts.length > 1) {
    return '.' + parts[parts.length - 1].toLowerCase()
  }
  return ''
}

/**
 * Base64 转 ArrayBuffer
 */
function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binaryString = atob(base64)
  const bytes = new Uint8Array(binaryString.length)
  for (let i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i)
  }
  return bytes.buffer
}

// 扩展 Window 接口类型声明
declare global {
  interface Window {
    showSaveFilePicker: (options?: {
      suggestedName?: string
      types?: Array<{
        description: string
        accept: Record<string, string[]>
      }>
    }) => Promise<FileSystemFileHandle>
  }
}