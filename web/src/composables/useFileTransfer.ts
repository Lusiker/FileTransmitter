import { ref, computed } from 'vue'
import type { Session, FileInfo } from '@/types'
import axios from 'axios'

// 动态获取API地址（支持移动设备IP访问）
function getApiBase(): string {
  const envBase = import.meta.env.VITE_API_BASE
  if (envBase) return envBase

  // 动态检测：使用当前访问的host + 后端端口8080
  const host = window.location.hostname
  return `http://${host}:8080`
}

const API_BASE = getApiBase()

const uploadingFiles = ref<Map<string, number>>(new Map()) // fileId -> progress
const downloadingFiles = ref<Map<string, number>>(new Map())

// 分片大小：5MB
const CHUNK_SIZE = 5 * 1024 * 1024

// 判断文件是否需要分片上传（大于 10MB）
function needsChunking(fileSize: number): boolean {
  return fileSize > 10 * 1024 * 1024
}

// Check if running on iOS/iPadOS
export function isIOS(): boolean {
  return /iPad|iPhone|iPod/.test(navigator.userAgent)
}

// Check if file type is previewable in Safari (by extension)
export function isPreviewable(filename: string): boolean {
  const ext = filename.split('.').pop()?.toLowerCase()
  const previewableExts = [
    'pdf',
    'png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp',
    'mp4', 'mov', 'm4v',
    'txt', 'json', 'csv'
  ]
  return previewableExts.includes(ext || '')
}

export function useFileTransfer() {
  const uploadProgress = computed(() => uploadingFiles.value)
  const downloadProgress = computed(() => downloadingFiles.value)

  // ==================== 核心下载函数（统一使用 <a> 下载）====================

  // 所有平台统一使用 <a> 或 window.open 下载
  // 让浏览器原生处理 Range 请求，避免内存问题
  function handleDownloadFile(
    sessionId: string,
    file: { id: string; name: string }
  ): void {
    const downloadUrl = `${API_BASE}/api/v1/transfer/${sessionId}/download/${file.id}`

    // 创建隐藏的 <a> 元素触发下载
    const link = document.createElement('a')
    link.href = downloadUrl
    link.download = file.name // 设置 download 属性触发下载而非预览
    link.style.display = 'none'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  // 预览文件（用于 PDF、图片等可预览类型）
  function previewFile(sessionId: string, fileId: string): void {
    const previewUrl = `${API_BASE}/api/v1/transfer/${sessionId}/download/${fileId}?inline=true`
    window.open(previewUrl, '_blank')
  }

  // 下载所有文件为 zip
  function downloadAllAsZip(sessionId: string): void {
    const url = `${API_BASE}/api/v1/transfer/${sessionId}/download/zip`
    window.location.href = url
  }

  // ==================== 会话管理 ====================

  async function createSession(
    senderId: string,
    receiverId: string,
    files: File[]
  ): Promise<Session> {
    console.log('[useFileTransfer] createSession called with files:', files.length)
    const fileInfos = files.map(f => ({
      name: f.name,
      size: f.size,
      mime_type: f.type,
      hash: ''
    }))
    console.log('[useFileTransfer] sending fileInfos:', fileInfos.length)

    const response = await axios.post(`${API_BASE}/api/v1/sessions`, {
      sender_id: senderId,
      receiver_id: receiverId,
      files: fileInfos
    })

    return response.data
  }

  async function acceptSession(sessionId: string, savePath: string): Promise<Session> {
    console.log('[useFileTransfer] Accepting session:', sessionId)
    try {
      const response = await axios.post(`${API_BASE}/api/v1/sessions/${sessionId}/accept`, {
        save_path: savePath
      }, {
        timeout: 30000  // 30 seconds timeout
      })
      console.log('[useFileTransfer] Session accepted:', response.data)
      return response.data
    } catch (error) {
      console.error('[useFileTransfer] Accept session failed:', error)
      throw error
    }
  }

  // ==================== 上传 ====================

  // 分片上传单个文件（用于大文件）
  async function uploadFileChunked(
    sessionId: string,
    file: File,
    fileId: string
  ): Promise<void> {
    const totalChunks = Math.ceil(file.size / CHUNK_SIZE)

    for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
      const start = chunkIndex * CHUNK_SIZE
      const end = Math.min(start + CHUNK_SIZE, file.size)
      const chunk = file.slice(start, end)

      const formData = new FormData()
      formData.append('session_id', sessionId)
      formData.append('file_id', fileId)
      formData.append('file', chunk, file.name)
      formData.append('chunk_index', String(chunkIndex))
      formData.append('total_chunks', String(totalChunks))
      formData.append('file_name', file.name)
      formData.append('file_size', String(file.size))

      try {
        await axios.post(`${API_BASE}/api/v1/transfer/upload/chunk`, formData, {
          timeout: 300000,  // 5 minutes timeout per chunk
          onUploadProgress: (progressEvent) => {
            if (progressEvent.total) {
              // 计算整体进度：已完成分片 + 当前分片进度
              const completedChunks = chunkIndex
              const chunkPercent = progressEvent.loaded / progressEvent.total
              const totalPercent = Math.round(((completedChunks + chunkPercent) / totalChunks) * 100)
              uploadingFiles.value.set(fileId, totalPercent)
            }
          }
        })
      } catch (error) {
        uploadingFiles.value.delete(fileId)
        throw error
      }
    }

    uploadingFiles.value.set(fileId, 100)
  }

  // 小文件整体上传
  async function uploadFileSmall(
    sessionId: string,
    file: File,
    fileId: string
  ): Promise<void> {
    const formData = new FormData()
    formData.append('session_id', sessionId)
    formData.append('file_id', fileId)
    formData.append('file', file)

    try {
      await axios.post(`${API_BASE}/api/v1/transfer/upload?session_id=${sessionId}&file_id=${fileId}`, formData, {
        timeout: 600000,
        onUploadProgress: (progressEvent) => {
          if (progressEvent.total) {
            const percent = Math.round((progressEvent.loaded / progressEvent.total) * 100)
            uploadingFiles.value.set(fileId, percent)
          }
        }
      })

      uploadingFiles.value.set(fileId, 100)
    } catch (error) {
      uploadingFiles.value.delete(fileId)
      throw error
    }
  }

  async function uploadFile(
    sessionId: string,
    file: File,
    fileId: string
  ): Promise<void> {
    // 根据文件大小选择上传方式
    if (needsChunking(file.size)) {
      return uploadFileChunked(sessionId, file, fileId)
    }
    return uploadFileSmall(sessionId, file, fileId)
  }

  async function uploadFiles(
    sessionId: string,
    files: File[],
    fileInfos: FileInfo[]
  ): Promise<void> {
    // 通过文件名匹配，而不是index
    const uploadPromises = files.map((file) => {
      const fileInfo = fileInfos.find(fi => fi.name === file.name)
      if (!fileInfo) {
        console.error(`File info not found for: ${file.name}`)
        return Promise.reject(`File info not found for: ${file.name}`)
      }
      return uploadFile(sessionId, file, fileInfo.id)
    })

    await Promise.all(uploadPromises)
  }

  // ==================== 其他 ====================

  async function cancelTransfer(sessionId: string): Promise<void> {
    await axios.post(`${API_BASE}/api/v1/transfer/${sessionId}/cancel`)
  }

  async function rejectSession(sessionId: string): Promise<void> {
    await axios.post(`${API_BASE}/api/v1/sessions/${sessionId}/reject`)
  }

  async function cleanupSessions(deviceId: string): Promise<void> {
    await axios.post(`${API_BASE}/api/v1/sessions/cleanup?device_id=${deviceId}`)
  }

  async function retryFiles(sessionId: string, fileIds: string[]): Promise<Session> {
    const response = await axios.post(`${API_BASE}/api/v1/sessions/${sessionId}/retry`, {
      file_ids: fileIds
    })
    return response.data
  }

  async function getTransferStatus(sessionId: string): Promise<any> {
    const response = await axios.get(`${API_BASE}/api/v1/transfer/${sessionId}/status`)
    return response.data
  }

  function clearProgress() {
    uploadingFiles.value.clear()
    downloadingFiles.value.clear()
  }

  return {
    uploadProgress,
    downloadProgress,
    createSession,
    acceptSession,
    uploadFile,
    uploadFiles,
    handleDownloadFile,
    previewFile,
    downloadAllAsZip,
    cancelTransfer,
    rejectSession,
    cleanupSessions,
    retryFiles,
    getTransferStatus,
    clearProgress,
    isIOS,
    isPreviewable
  }
}