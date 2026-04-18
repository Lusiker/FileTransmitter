import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Session, FileStatus } from '@/types'
import axios from 'axios'

export const useSessionStore = defineStore('session', () => {
  const sessions = ref<Session[]>([])
  const activeSession = ref<Session | null>(null)

  const pendingSessions = computed(() => {
    return sessions.value.filter(s => s.state === 'pending')
  })

  const activeSessions = computed(() => {
    return sessions.value.filter(s =>
      s.state === 'transferring' || s.state === 'accepted'
    )
  })

  const completedSessions = computed(() => {
    return sessions.value.filter(s =>
      s.state === 'completed' || s.state === 'partially_completed'
    )
  })

  const failedFiles = computed(() => {
    if (!activeSession.value) return []
    return activeSession.value.files.filter(f => f.status === 'failed')
  })

  function handleWSMessage(event: MessageEvent) {
    try {
      const message = JSON.parse(event.data)
      switch (message.type) {
        case 'session_created':
          addSession(message.data)
          break
        case 'session_accepted':
          updateSession(message.data)
          break
        case 'transfer_progress':
          updateProgress(message.data)
          break
        case 'file_complete':
          updateFileStatus(message.data.session_id, message.data.file_id, 'success')
          break
        case 'file_failed':
          updateFileStatus(
            message.data.session_id,
            message.data.file_id,
            'failed',
            message.data.error
          )
          break
        case 'transfer_complete':
          updateSession(message.data)
          break
        case 'transfer_failed':
          updateSession(message.data)
          break
      }
    } catch (error) {
      console.error('Failed to handle WS message:', error)
    }
  }

  function addSession(session: Session) {
    const index = sessions.value.findIndex(s => s.id === session.id)
    if (index >= 0) {
      sessions.value[index] = session
    } else {
      sessions.value.push(session)
    }
    // 只有非 pending 状态才设为 activeSession
    // pending 状态需要用户确认，应该由 incomingSessions 处理
    if (session.state !== 'pending' && !activeSession.value) {
      activeSession.value = session
    }
  }

  function updateSession(session: Session) {
    const index = sessions.value.findIndex(s => s.id === session.id)
    if (index >= 0) {
      sessions.value[index] = session
    }
    if (activeSession.value?.id === session.id) {
      activeSession.value = session
    }
  }

  function updateProgress(data: { session_id: string; file_id: string; bytes: number; percent: number; phase?: string }) {
    const session = sessions.value.find(s => s.id === data.session_id)
    if (!session) return

    const file = session.files.find(f => f.id === data.file_id)
    if (file) {
      file.transfer_size = data.bytes
      // 只有 uploading phase 才设置 transferring 状态
      // merging phase 保持当前进度（100%），不覆盖
      if (data.phase === 'uploading') {
        file.status = 'transferring'
      } else if (data.phase === 'merging') {
        file.status = 'merging'  // 新增合并中状态
      } else if (data.phase === 'done') {
        file.status = 'success'
        file.transfer_size = file.size
      }
    }

    session.transferred = session.files.reduce((sum, f) => sum + f.transfer_size, 0)

    if (activeSession.value?.id === data.session_id) {
      activeSession.value = { ...session }
    }
  }

  function updateFileStatus(sessionId: string, fileId: string, status: FileStatus, error?: string) {
    const session = sessions.value.find(s => s.id === sessionId)
    if (!session) return

    const file = session.files.find(f => f.id === fileId)
    if (file) {
      file.status = status
      if (error) file.error = error
      if (status === 'success') {
        file.transfer_size = file.size
      }
    }

    if (activeSession.value?.id === sessionId) {
      activeSession.value = { ...session }
    }
  }

  function setActiveSession(session: Session | null) {
    activeSession.value = session
  }

  function getSession(id: string): Session | undefined {
    return sessions.value.find(s => s.id === id)
  }

  function removeSession(id: string) {
    const index = sessions.value.findIndex(s => s.id === id)
    if (index >= 0) {
      sessions.value.splice(index, 1)
    }
    if (activeSession.value?.id === id) {
      activeSession.value = null
    }
  }

  function clearSessions() {
    sessions.value = []
    activeSession.value = null
  }

  // ==================== 流式传输消息处理 ====================

  // 流式接收回调（由外部 useStreamReceiver 设置）
  let onChunkDataCallback: ((data: any) => Promise<void>) | null = null
  let onFileReadyCallback: ((data: any) => Promise<void>) | null = null

  function setOnChunkDataCallback(callback: (data: any) => Promise<void>) {
    onChunkDataCallback = callback
  }

  function setOnFileReadyCallback(callback: (data: any) => Promise<void>) {
    onFileReadyCallback = callback
  }

  // 处理接收到的分片数据
  async function handleChunkData(data: any) {
    if (onChunkDataCallback) {
      await onChunkDataCallback(data)
    } else {
      console.warn('[Session] No chunk data callback set')
    }
  }

  // 处理 ACK（发送端收到）
  function handleChunkAck(data: any) {
    // 更新发送进度
    const session = sessions.value.find(s => s.id === data.session_id)
    if (session) {
      const file = session.files.find(f => f.id === data.file_id)
      if (file && data.status === 'ok') {
        // 分片已确认，进度已更新（由 useFileTransfer 处理）
        console.log('[Session] Chunk ACK:', data.file_id, data.chunk_index)
      }
    }
  }

  // 处理文件准备开始传输
  async function handleFileReady(data: any) {
    console.log('[Session] File ready:', data.file_id)
    if (onFileReadyCallback) {
      await onFileReadyCallback(data)
    }
  }

  // 处理文件开始传输通知（发送端）
  function handleFileStart(data: any) {
    console.log('[Session] File start:', data.file_id)
    // 由 useFileTransfer 处理，开始传输下一个文件
  }

  // 处理文件传输结束
  function handleFileEnd(data: any) {
    updateFileStatus(data.session_id, data.file_id, data.success ? 'success' : 'failed', data.error)
  }

  // 处理接收端准备好
  function handleReceiverReady(data: any) {
    console.log('[Session] Receiver ready, supports streaming:', data.supports_streaming)
    // 由 useFileTransfer 处理，决定传输方式
  }

  // 处理接收端离线
  function handleReceiverOffline(data: any) {
    console.log('[Session] Receiver offline:', data.session_id)
    // 暂停传输或切换到 fallback
  }

  // 从后端同步当前设备相关的 session
  async function syncSessions(deviceId: string) {
    try {
      const response = await axios.get('/api/v1/sessions')
      const allSessions: Session[] = response.data.sessions || []

      // 过滤出与当前设备相关的 sessions
      const mySessions = allSessions.filter(s =>
        s.sender_id === deviceId || s.receiver_id === deviceId
      )

      // 更新 sessions 列表
      for (const session of mySessions) {
        addSession(session)
      }

      console.log('[Session] Synced', mySessions.length, 'sessions for device', deviceId)
    } catch (error) {
      console.error('[Session] Failed to sync sessions:', error)
    }
  }

  return {
    sessions,
    activeSession,
    pendingSessions,
    activeSessions,
    completedSessions,
    failedFiles,
    handleWSMessage,
    addSession,
    updateSession,
    updateProgress,
    updateFileStatus,
    setActiveSession,
    getSession,
    removeSession,
    clearSessions,
    syncSessions,
    // 流式传输
    setOnChunkDataCallback,
    setOnFileReadyCallback,
    handleChunkData,
    handleChunkAck,
    handleFileReady,
    handleFileStart,
    handleFileEnd,
    handleReceiverReady,
    handleReceiverOffline
  }
})