import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Device, DeviceStatusData } from '@/types'
import { toDevice } from '@/types'
import { useSessionStore } from './session'

// 动态获取WebSocket地址（支持移动设备IP访问）
function getWsHost(): string {
  const envBase = import.meta.env.VITE_API_BASE
  if (envBase) {
    return envBase.replace(/^http/, 'ws')
  }

  // 动态检测：使用当前访问的host + 后端端口8080
  const host = window.location.hostname
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${host}:8080`
}

export const useDeviceStore = defineStore('device', () => {
  const deviceId = ref<string>('')
  const deviceName = ref<string>('')
  const deviceRole = ref<'sender' | 'receiver'>('sender')
  const deviceIP = ref<string>('')

  // 其他发现的设备
  const devices = ref<Device[]>([])

  // 过滤掉自己，只显示其他设备
  const discoveredDevices = computed(() => {
    return devices.value.filter(d => d.id !== deviceId.value)
  })

  const senderDevices = computed(() => {
    return discoveredDevices.value.filter(d => d.role === 'sender')
  })

  const receiverDevices = computed(() => {
    return discoveredDevices.value.filter(d => d.role === 'receiver')
  })

  const isConnected = ref(false)

  // WebSocket 连接
  let wsConnection: WebSocket | null = null
  let manualDisconnect = false  // 标记是否主动断开（不触发自动重连）

  async function initDevice() {
    try {
      // 角色优先从 URL 路径判断（比 sessionStorage 更可靠）
      const path = window.location.pathname
      if (path.includes('/receiver')) {
        deviceRole.value = 'receiver'
      } else if (path.includes('/sender')) {
        deviceRole.value = 'sender'
      } else {
        // 从 sessionStorage 恢复角色
        deviceRole.value = (sessionStorage.getItem('deviceRole') as 'sender' | 'receiver') || 'sender'
      }

      // 根据角色获取/生成 deviceId
      const roleKey = deviceRole.value === 'sender' ? 'deviceId_sender' : 'deviceId_receiver'
      const nameKey = deviceRole.value === 'sender' ? 'deviceName_sender' : 'deviceName_receiver'

      deviceId.value = localStorage.getItem(roleKey) || generateId()

      // 检查名称是否是旧格式，如果是则重新生成
      const savedName = localStorage.getItem(nameKey)
      const oldFormatNames = ['电脑', 'Android手机', 'iPhone', '移动设备']
      if (savedName && !oldFormatNames.includes(savedName)) {
        deviceName.value = savedName
      } else {
        // 生成新的随机名称
        deviceName.value = getDefaultName()
      }

      // 保存
      localStorage.setItem(roleKey, deviceId.value)
      localStorage.setItem(nameKey, deviceName.value)
      sessionStorage.setItem('deviceRole', deviceRole.value)

      // 连接 WebSocket
      connectWebSocket()
    } catch (error) {
      console.error('Failed to initialize device:', error)
    }
  }

  function getDefaultName(): string {
    // 生成6位16进制字符串作为默认名称
    const hex = Math.random().toString(16).substring(2, 8).toUpperCase()
    return hex
  }

  function setName(name: string) {
    deviceName.value = name
    // 按角色保存到 localStorage
    const nameKey = deviceRole.value === 'sender' ? 'deviceName_sender' : 'deviceName_receiver'
    localStorage.setItem(nameKey, name)
    // 重连 WebSocket 以更新名称（无论当前是否连接）
    manualDisconnect = true
    disconnect()
    manualDisconnect = false
    connectWebSocket()
  }

  function setRole(role: 'sender' | 'receiver') {
    deviceRole.value = role
    sessionStorage.setItem('deviceRole', role)

    // 切换角色时需要重新获取对应的 deviceId 和 name
    const roleKey = role === 'sender' ? 'deviceId_sender' : 'deviceId_receiver'
    const nameKey = role === 'sender' ? 'deviceName_sender' : 'deviceName_receiver'

    deviceId.value = localStorage.getItem(roleKey) || generateId()
    deviceName.value = localStorage.getItem(nameKey) || getDefaultName()

    localStorage.setItem(roleKey, deviceId.value)
    localStorage.setItem(nameKey, deviceName.value)

    // 重连 WebSocket 以更新角色和设备信息（无论当前是否连接）
    manualDisconnect = true
    disconnect()
    // 立即重新连接
    manualDisconnect = false
    connectWebSocket()
  }

  function handleWSMessage(event: MessageEvent) {
    try {
      // 后端可能批量发送多条消息（用换行符分隔）
      const messages = event.data.split('\n').filter((msg: string) => msg.trim())

      for (const msg of messages) {
        try {
          const message = JSON.parse(msg)
          console.log('[WS] Message:', message.type, message.data)

          // Device-related messages
          switch (message.type) {
            case 'device_online':
              addDevice(toDevice(message.data))
              break
            case 'device_offline':
              removeDevice(message.data.device_id)
              break
            case 'device_list':
              console.log('[WS] Received device_list:', message.data)
              if (Array.isArray(message.data)) {
                message.data.forEach((d: DeviceStatusData) => addDevice(toDevice(d)))
              }
              break
            case 'client_ip':
              deviceIP.value = message.data.ip
              console.log('[WS] Got client IP:', message.data.ip)
              break
          }

          // Session-related messages - forward to session store
          const sessionStore = useSessionStore()
          switch (message.type) {
            case 'session_created':
              sessionStore.addSession(message.data)
              break
            case 'session_accepted':
              sessionStore.updateSession(message.data)
              break
            case 'session_cancelled':
              sessionStore.removeSession(message.data.id)
              break
            case 'session_rejected':
              sessionStore.removeSession(message.data.id)
              break
            case 'transfer_progress':
              sessionStore.updateProgress(message.data)
              break
            case 'file_complete':
              sessionStore.updateFileStatus(message.data.session_id, message.data.file_id, 'success')
              break
            case 'file_failed':
              sessionStore.updateFileStatus(message.data.session_id, message.data.file_id, 'failed', message.data.error)
              break
            case 'transfer_complete':
              sessionStore.updateSession(message.data)
              break

            // 流式传输消息
            case 'chunk_data':
              // 由 useStreamReceiver 处理
              sessionStore.handleChunkData(message.data)
              break
            case 'chunk_ack':
              // 发送端收到 ACK，继续下一个分片
              sessionStore.handleChunkAck(message.data)
              break
            case 'file_ready':
              // 文件准备开始传输
              sessionStore.handleFileReady(message.data)
              break
            case 'file_start':
              // 通知发送端开始传输文件
              sessionStore.handleFileStart(message.data)
              break
            case 'file_end':
              // 文件传输结束
              sessionStore.handleFileEnd(message.data)
              break
            case 'receiver_ready':
              // 接收端准备好
              sessionStore.handleReceiverReady(message.data)
              break
            case 'receiver_offline':
              // 接收端离线
              sessionStore.handleReceiverOffline(message.data)
              break
          }
        } catch (parseError) {
          console.error('[WS] Failed to parse message:', msg, parseError)
        }
      }
    } catch (error) {
      console.error('Failed to handle WS message:', error)
    }
  }

  function addDevice(device: Device) {
    if (device.id === deviceId.value) return

    const index = devices.value.findIndex(d => d.id === device.id)
    if (index >= 0) {
      devices.value[index] = device
    } else {
      devices.value.push(device)
    }
  }

  function removeDevice(id: string) {
    devices.value = devices.value.filter(d => d.id !== id)
  }

  function clearDevices() {
    devices.value = []
  }

  function getDevice(id: string): Device | undefined {
    return devices.value.find(d => d.id === id)
  }

  function generateId(): string {
    return 'device-' + Math.random().toString(36).substring(2, 9)
  }

  function connectWebSocket() {
    // 确保没有重复连接
    if (wsConnection) {
      manualDisconnect = true
      disconnect()
    }

    manualDisconnect = false  // 重置标志，允许自动重连

    const wsHost = getWsHost()
    const wsUrl = `${wsHost}/api/v1/ws?device_id=${deviceId.value}&name=${encodeURIComponent(deviceName.value)}&role=${deviceRole.value}`

    try {
      wsConnection = new WebSocket(wsUrl)

      wsConnection.onopen = () => {
        isConnected.value = true
        console.log('[WS] Connected as', deviceName.value, '(' + deviceRole.value + ')')
      }

      wsConnection.onmessage = handleWSMessage

      wsConnection.onclose = () => {
        isConnected.value = false
        wsConnection = null
        console.log('[WS] Disconnected, manualDisconnect:', manualDisconnect)
        // 只有非主动断开时才自动重连
        if (!manualDisconnect) {
          setTimeout(() => {
            if (!isConnected.value && !manualDisconnect) {
              connectWebSocket()
            }
          }, 3000)
        }
      }

      wsConnection.onerror = (error) => {
        console.error('[WS] Error:', error)
        isConnected.value = false
      }
    } catch (error) {
      console.error('[WS] Connection failed:', error)
    }
  }

  function disconnect() {
    manualDisconnect = true
    if (wsConnection) {
      wsConnection.close()
      wsConnection = null
    }
    isConnected.value = false
  }

  // 获取 WebSocket 连接（用于发送消息）
  function getWebSocketConnection(): WebSocket | null {
    return wsConnection
  }

  // 发送 WebSocket 消息
  function sendWSMessage(type: string, data: any): boolean {
    if (!wsConnection || wsConnection.readyState !== WebSocket.OPEN) {
      console.warn('[WS] Cannot send message, not connected')
      return false
    }

    const message = JSON.stringify({ type, data })
    wsConnection.send(message)
    return true
  }

  return {
    deviceId,
    deviceName,
    deviceRole,
    deviceIP,
    devices,
    discoveredDevices,
    senderDevices,
    receiverDevices,
    isConnected,
    initDevice,
    setName,
    setRole,
    addDevice,
    removeDevice,
    clearDevices,
    getDevice,
    disconnect,
    getWebSocketConnection,
    sendWSMessage
  }
})