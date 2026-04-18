<template>
  <div class="sender-page">
    <header class="page-header">
      <div class="header-row header-main">
        <el-button @click="goBack" circle size="small">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <h1>发送文件</h1>
        <div class="device-info">
          <el-tag type="info" size="small">{{ deviceStore.deviceName }}</el-tag>
          <el-tag type="info" size="small">{{ deviceStore.deviceIP || '...' }}</el-tag>
        </div>
      </div>
      <div class="header-row header-status">
        <el-tag :type="deviceStore.isConnected ? 'success' : 'danger'" size="small">
          {{ deviceStore.isConnected ? '在线' : '离线' }}
        </el-tag>
      </div>
    </header>

    <main class="page-main">
      <!-- Orphaned pending session warning -->
      <section v-if="hasOrphanedPending" class="orphaned-warning">
        <el-alert type="warning" :closable="false">
          <template #title>
            发现未完成的传输请求
          </template>
          <template #default>
            <p>页面刷新后文件信息丢失，请重新选择文件</p>
            <el-button type="primary" size="small" @click="cancelOrphaned">
              取消请求
            </el-button>
          </template>
        </el-alert>
      </section>

      <!-- Waiting for acceptance -->
      <section v-if="isWaitingForAcceptance" class="waiting-section">
        <el-card>
          <div class="waiting-content">
            <el-icon size="48" class="waiting-icon"><Loading /></el-icon>
            <h3>等待接收者确认</h3>
            <p>{{ pendingFiles.length }} 个文件准备发送</p>
            <p class="hint">接收者确认后将自动开始传输</p>
          </div>
        </el-card>
      </section>

      <!-- File Selection -->
      <section v-if="!isWaitingForAcceptance" class="file-section">
        <div class="section-header">
          <h2>选择文件</h2>
          <el-button
            v-if="selectedFiles.length > 0"
            type="danger"
            size="small"
            @click="clearSelectedFiles"
          >
            清空
          </el-button>
        </div>
        <FileSelector
          v-model="selectedFiles"
          @change="onFileChange"
        />
      </section>

      <!-- Receiver Selection -->
      <section v-if="!isWaitingForAcceptance" class="receiver-section">
        <h2>选择接收者</h2>
        <ReceiverList
          :receivers="deviceStore.receiverDevices"
          :selected-receiver="selectedReceiver"
          @update:selected-receiver="selectedReceiver = $event"
        />
      </section>

      <!-- Transfer Queue -->
      <section v-if="sessionStore.activeSession && sessionStore.activeSession.state !== 'pending'" class="transfer-section">
        <div class="section-header">
          <h2>传输队列</h2>
          <el-button
            v-if="sessionStore.activeSession?.state === 'completed'"
            type="success"
            size="small"
            @click="resetForNextTransfer"
          >
            新传输
          </el-button>
        </div>
        <TransferQueue
          :session="sessionStore.activeSession"
          @cancel="handleCancelTransfer"
          @retry="handleRetryFiles"
        />
      </section>

      <!-- Failed Files -->
      <section v-if="sessionStore.failedFiles.length > 0" class="failed-section">
        <h2>失败文件</h2>
        <FailedFiles
          :files="sessionStore.failedFiles"
          :session-id="sessionStore.activeSession?.id"
          @retry="handleRetryFiles"
        />
      </section>
    </main>

    <footer class="page-footer">
      <el-button
        v-if="!isWaitingForAcceptance"
        type="primary"
        size="large"
        :disabled="!canStartTransfer"
        @click="startTransfer"
      >
        发送请求
      </el-button>
      <el-button
        v-if="isWaitingForAcceptance"
        type="info"
        size="large"
        @click="cancelPending"
      >
        取消等待
      </el-button>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useDeviceStore } from '@/stores/device'
import { useSessionStore } from '@/stores/session'
import { useFileTransfer } from '@/composables/useFileTransfer'
import FileSelector from './FileSelector.vue'
import ReceiverList from './ReceiverList.vue'
import TransferQueue from './TransferQueue.vue'
import FailedFiles from './FailedFiles.vue'
import type { Device } from '@/types'

const router = useRouter()
const deviceStore = useDeviceStore()
const sessionStore = useSessionStore()
const { createSession, uploadFiles, cancelTransfer, retryFiles } = useFileTransfer()

const selectedFiles = ref<File[]>([])
const selectedReceiver = ref<Device | null>(null)
const pendingSessionId = ref<string | null>(null)
const pendingFiles = ref<File[]>([])
const uploadStarted = ref(false)

const restoredPendingId = sessionStorage.getItem('pendingSessionId')
if (restoredPendingId) {
  pendingSessionId.value = restoredPendingId
  sessionStorage.removeItem('pendingSessionId')
  sessionStorage.removeItem('pendingFiles')
}

const canStartTransfer = computed(() => {
  return selectedFiles.value.length > 0 && selectedReceiver.value && deviceStore.isConnected
})

const isWaitingForAcceptance = computed(() => {
  return pendingSessionId.value !== null &&
    sessionStore.activeSession?.id === pendingSessionId.value &&
    sessionStore.activeSession?.state === 'pending'
})

const hasOrphanedPending = computed(() => {
  const pending = sessionStore.pendingSessions.find(s => s.sender_id === deviceStore.deviceId)
  return pending && pendingFiles.value.length === 0 && pendingSessionId.value !== pending.id
})

function goBack() {
  router.push('/')
}

function onFileChange(files: File[]) {
  selectedFiles.value = files
}

function clearSelectedFiles() {
  selectedFiles.value = []
}

function resetForNextTransfer() {
  pendingSessionId.value = null
  pendingFiles.value = []
  uploadStarted.value = false
  sessionStore.setActiveSession(null)
  sessionStorage.removeItem('pendingSessionId')
  sessionStorage.removeItem('pendingFiles')
}

async function startTransfer() {
  if (!selectedReceiver.value || selectedFiles.value.length === 0) return

  try {
    const session = await createSession(
      deviceStore.deviceId,
      selectedReceiver.value.id,
      selectedFiles.value
    )

    sessionStore.addSession(session)
    sessionStore.setActiveSession(session)

    pendingSessionId.value = session.id
    pendingFiles.value = selectedFiles.value

    sessionStorage.setItem('pendingSessionId', session.id)
    sessionStorage.setItem('pendingFiles', JSON.stringify(selectedFiles.value.map(f => f.name)))

    selectedFiles.value = []
    selectedReceiver.value = null
  } catch (error) {
    console.error('Failed to create session:', error)
  }
}

function cancelPending() {
  if (pendingSessionId.value) {
    cancelTransfer(pendingSessionId.value)
    pendingSessionId.value = null
    pendingFiles.value = []
    sessionStore.setActiveSession(null)
    sessionStorage.removeItem('pendingSessionId')
    sessionStorage.removeItem('pendingFiles')
  }
}

function cancelOrphaned() {
  const orphaned = sessionStore.pendingSessions.find(s => s.sender_id === deviceStore.deviceId)
  if (orphaned) {
    cancelTransfer(orphaned.id)
    sessionStore.removeSession(orphaned.id)
  }
}

async function handleCancelTransfer() {
  if (sessionStore.activeSession) {
    await cancelTransfer(sessionStore.activeSession.id)
    pendingSessionId.value = null
    pendingFiles.value = []
    sessionStorage.removeItem('pendingSessionId')
    sessionStorage.removeItem('pendingFiles')
  }
}

async function handleRetryFiles(fileIds: string[]) {
  if (sessionStore.activeSession) {
    await retryFiles(sessionStore.activeSession.id, fileIds)
    if (pendingFiles.value.length > 0) {
      const session = sessionStore.activeSession
      const filesToRetry = pendingFiles.value.filter(f => {
        const fileInfo = session.files.find(fi => fi.name === f.name)
        return fileInfo && fileIds.includes(fileInfo.id)
      })
      if (filesToRetry.length > 0) {
        uploadFiles(session.id, filesToRetry, session.files)
          .catch(err => console.error('[Sender] Retry upload failed:', err))
      }
    }
  }
}

watch(() => sessionStore.activeSession?.state, (state, oldState) => {
  const session = sessionStore.activeSession
  if (session && pendingSessionId.value === session.id &&
      oldState !== 'accepted' && state === 'accepted' && !uploadStarted.value) {
    if (pendingFiles.value.length > 0) {
      uploadStarted.value = true
      uploadFiles(session.id, pendingFiles.value, session.files)
        .then(() => {
          pendingSessionId.value = null
          pendingFiles.value = []
          uploadStarted.value = false
          sessionStorage.removeItem('pendingSessionId')
          sessionStorage.removeItem('pendingFiles')
        })
        .catch((error) => {
          console.error('[Sender] Upload failed:', error)
          uploadStarted.value = false
        })
    }
  }
})

onMounted(async () => {
  await sessionStore.syncSessions(deviceStore.deviceId)

  const myPending = sessionStore.pendingSessions.find(s => s.sender_id === deviceStore.deviceId)
  if (myPending && pendingFiles.value.length === 0) {
    console.log('[Sender] Found orphaned pending session:', myPending.id)
  }
})
</script>

<style lang="scss" scoped>
.sender-page {
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

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;

    h2 {
      margin-bottom: 0;
    }
  }
}

.orphaned-warning {
  p {
    margin: 8px 0;
    font-size: 0.85rem;
  }
}

.page-footer {
  padding: 12px 16px;
  background: white;
  box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.1);
  display: flex;
  justify-content: center;
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

  .page-footer {
    padding: 16px 24px;
  }
}
</style>