<template>
  <div class="receiver-page">
    <header class="page-header">
      <div class="header-row header-main">
        <el-button @click="goBack" circle size="small">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <h1>接收文件</h1>
        <div class="device-info">
          <el-tag type="info" size="small">{{ deviceStore.deviceName }}</el-tag>
          <el-tag type="info" size="small">{{ deviceStore.deviceIP || '...' }}</el-tag>
        </div>
      </div>
      <div class="header-row header-status">
        <el-tag :type="deviceStore.isConnected ? 'success' : 'danger'" size="small">
          {{ deviceStore.isConnected ? '在线' : '离线' }}
        </el-tag>
        <el-button
          v-if="hasAnyTask"
          type="warning"
          size="small"
          @click="clearAllTasks"
        >
          清空任务
        </el-button>
      </div>
    </header>

    <main class="page-main">
      <!-- Sender Discovery -->
      <section class="sender-section">
        <h2>发现发送者</h2>
        <SenderDiscovery
          :senders="deviceStore.senderDevices"
          :selected-sender="selectedSender"
          @update:selected-sender="selectedSender = $event"
          @refresh="refreshDevices"
        />
      </section>

      <!-- Waiting for transfer when sender selected -->
      <section v-if="selectedSender && !incomingSessions.length && !myActiveSession" class="waiting-section">
        <el-card>
          <div class="waiting-content">
            <el-icon size="48" class="waiting-icon"><Loading /></el-icon>
            <h3>等待发送者发送文件</h3>
            <p>已选择: {{ selectedSender.name }}</p>
            <p class="hint">发送者选择文件后将自动显示传输请求</p>
          </div>
        </el-card>
      </section>

      <!-- Incoming Transfers -->
      <section v-if="incomingSessions.length > 0 && !myActiveSession" class="incoming-section">
        <h2>待接收传输</h2>
        <IncomingTransfer
          :sessions="incomingSessions"
          @accept="acceptTransfer"
        />
      </section>

      <!-- Active Transfer -->
      <section v-if="myActiveSession" class="transfer-section">
        <h2>当前传输</h2>
        <TransferQueue
          :session="myActiveSession"
          @cancel="handleCancelTransfer"
          @retry="handleRetryFiles"
        />
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useDeviceStore } from '@/stores/device'
import { useSessionStore } from '@/stores/session'
import { useFileTransfer } from '@/composables/useFileTransfer'
import { ElMessage } from 'element-plus'
import SenderDiscovery from './SenderDiscovery.vue'
import IncomingTransfer from './IncomingTransfer.vue'
import TransferQueue from '@/views/Sender/TransferQueue.vue'
import type { Device, Session } from '@/types'

const router = useRouter()
const deviceStore = useDeviceStore()
const sessionStore = useSessionStore()
const { acceptSession, cancelTransfer, retryFiles, rejectSession, cleanupSessions } = useFileTransfer()

const selectedSender = ref<Device | null>(null)

const incomingSessions = computed(() => {
  return sessionStore.pendingSessions.filter(s => s.receiver_id === deviceStore.deviceId)
})

const myActiveSession = computed(() => {
  const acceptedSessions = sessionStore.sessions.filter(s =>
    s.receiver_id === deviceStore.deviceId &&
    (s.state === 'accepted' || s.state === 'transferring')
  )
  if (acceptedSessions.length > 0) {
    return acceptedSessions[0]
  }
  const session = sessionStore.activeSession
  if (session && session.receiver_id === deviceStore.deviceId && session.state !== 'pending') {
    return session
  }
  return null
})

const hasAnyTask = computed(() => {
  return incomingSessions.value.length > 0 || myActiveSession.value !== null
})

function goBack() {
  router.push('/')
}

function refreshDevices() {
  console.log('Refreshing devices...')
}

async function acceptTransfer(session: Session) {
  try {
    console.log('[Receiver] Accepting transfer:', session.id)
    const acceptedSession = await acceptSession(session.id, './downloads')
    sessionStore.updateSession(acceptedSession)
    sessionStore.setActiveSession(acceptedSession)
    ElMessage.success('已接受传输请求')
  } catch (error: any) {
    console.error('Failed to accept transfer:', error)
    ElMessage.error(error.response?.data?.error || '接受传输失败')
  }
}

async function handleCancelTransfer() {
  if (sessionStore.activeSession) {
    await cancelTransfer(sessionStore.activeSession.id)
    sessionStore.setActiveSession(null)
  }
}

async function handleRetryFiles(fileIds: string[]) {
  if (sessionStore.activeSession) {
    await retryFiles(sessionStore.activeSession.id, fileIds)
  }
}

async function clearAllTasks() {
  for (const session of incomingSessions.value) {
    try {
      await rejectSession(session.id)
    } catch (error) {
      console.error('Failed to reject session:', error)
    }
  }

  if (myActiveSession.value) {
    try {
      await cancelTransfer(myActiveSession.value.id)
    } catch (error) {
      console.error('Failed to cancel transfer:', error)
    }
  }

  sessionStore.clearSessions()

  try {
    await cleanupSessions(deviceStore.deviceId)
  } catch (error) {
    console.error('Failed to cleanup sessions:', error)
  }
}

watch(myActiveSession, (session) => {
  if (session && !sessionStore.activeSession) {
    sessionStore.setActiveSession(session)
  }
}, { immediate: true })

onMounted(async () => {
  await sessionStore.syncSessions(deviceStore.deviceId)

  if (myActiveSession.value) {
    sessionStore.setActiveSession(myActiveSession.value)
  }
})
</script>

<style lang="scss" scoped>
.receiver-page {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: #f5f7fa;
}

.page-header {
  display: flex;
  flex-direction: column;
  background: white;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  padding: 12px 16px;
  gap: 8px;

  .header-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .header-main {
    h1 {
      font-size: 1.25rem;
      color: #303133;
      margin: 0;
      white-space: nowrap;
    }

    .device-info {
      display: flex;
      gap: 6px;
      margin-left: auto;
    }
  }

  .header-status {
    justify-content: flex-end;
  }
}

.page-main {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;

  section {
    h2 {
      color: #303133;
      margin-bottom: 12px;
      font-size: 1.1rem;
    }
  }
}

.waiting-section {
  .waiting-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 24px 16px;

    .waiting-icon {
      color: #409EFF;
      animation: spin 1s linear infinite;
    }

    h3 {
      color: #303133;
      margin: 12px 0 8px;
      font-size: 1rem;
    }

    .hint {
      color: #909399;
      font-size: 0.8rem;
      text-align: center;
    }
  }
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@media (min-width: 600px) {
  .page-header {
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
    padding: 16px 24px;

    .header-main {
      h1 {
        font-size: 1.5rem;
      }
    }

    .header-status {
      justify-content: flex-start;
    }
  }

  .page-main {
    padding: 24px;
    gap: 24px;

    section h2 {
      font-size: 1.25rem;
      margin-bottom: 16px;
    }
  }
}
</style>