<template>
  <div class="incoming-transfer">
    <div v-for="session in sessions" :key="session.id" class="transfer-card">
      <div class="transfer-header">
        <h4>来自: {{ getSenderName(session.sender_id) }}</h4>
        <el-tag :type="getSessionType(session.state)">
          {{ session.state }}
        </el-tag>
      </div>

      <div class="transfer-files">
        <p>{{ session.files.length }} 个文件 ({{ formatSize(session.total_size) }})</p>
        <div class="file-preview">
          <span v-for="(file, index) in session.files.slice(0, 3)" :key="file.id">
            {{ file.name }}
            {{ index < Math.min(session.files.length - 1, 2) ? ',' : '' }}
          </span>
          <span v-if="session.files.length > 3">
            ... 等 {{ session.files.length }} 个文件
          </span>
        </div>
      </div>

      <div class="transfer-actions">
        <el-button type="success" @click="$emit('accept', session)">
          接收
        </el-button>
        <el-button type="info" @click="viewDetails(session)">
          查看详情
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useDeviceStore } from '@/stores/device'
import type { Session } from '@/types'

defineProps<{
  sessions: Session[]
}>()

defineEmits<{
  accept: [session: Session]
}>()

const deviceStore = useDeviceStore()

function getSenderName(senderId: string): string {
  const device = deviceStore.getDevice(senderId)
  return device?.name || 'Unknown'
}

function getSessionType(state: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  switch (state) {
    case 'pending':
      return 'warning'
    case 'accepted':
      return 'success'
    case 'transferring':
      return ''
    case 'completed':
      return 'success'
    case 'failed':
      return 'danger'
    default:
      return 'info'
  }
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function viewDetails(_session: Session) {
  // Could open a dialog with full file list
}
</script>

<style lang="scss" scoped>
.incoming-transfer {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.transfer-card {
  background: white;
  border-radius: 8px;
  padding: 16px;
  border: 2px solid #EBEEF5;
}

.transfer-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;

  h4 {
    color: #303133;
  }
}

.transfer-files {
  margin-bottom: 16px;

  p {
    color: #606266;
    margin-bottom: 8px;
  }

  .file-preview {
    color: #909399;
    font-size: 0.875rem;
  }
}

.transfer-actions {
  display: flex;
  gap: 8px;
}
</style>