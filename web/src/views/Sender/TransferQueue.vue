<template>
  <div class="transfer-queue">
    <!-- iPad 保存提示 -->
    <IOSGuide />

    <div class="queue-header">
      <span>传输进度</span>
      <el-button
        v-if="session.state === 'transferring'"
        type="danger"
        size="small"
        @click="$emit('cancel')"
      >
        取消
      </el-button>
      <el-button
        v-if="session.state === 'completed' || session.state === 'partially_completed'"
        type="primary"
        size="small"
        @click="downloadAll"
      >
        下载全部
      </el-button>
    </div>

    <div class="overall-progress">
      <el-progress
        :percentage="overallPercent"
        :status="progressStatus"
      />
      <div class="progress-info">
        <span>{{ formatSize(session.transferred) }} / {{ formatSize(session.total_size) }}</span>
        <span>{{ stateText }}</span>
      </div>
    </div>

    <div class="file-list">
      <div
        v-for="file in session.files"
        :key="file.id"
        class="file-item"
      >
        <div class="file-name">
          <el-icon><Document /></el-icon>
          <span>{{ file.name }}</span>
        </div>
        <div class="file-progress">
          <el-progress
            :percentage="filePercent(file)"
            :status="fileStatus(file.status)"
            :stroke-width="6"
          />
        </div>
        <div class="file-size">
          {{ formatSize(file.size) }}
        </div>
        <StatusBadge :status="file.status" :error="file.error" />
        <!-- 下载按钮 -->
        <el-button
          v-if="file.status === 'success'"
          type="primary"
          size="small"
          link
          @click="downloadFile(file)"
        >
          <el-icon><Download /></el-icon>
          下载
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Session, FileInfo } from '@/types'
import StatusBadge from '@/components/common/StatusBadge.vue'
import IOSGuide from '@/components/common/IOSGuide.vue'
import { useFileTransfer, isIOS } from '@/composables/useFileTransfer'

const props = defineProps<{
  session: Session
}>()

defineEmits<{
  cancel: []
  retry: [fileIds: string[]]
}>()

const { handleDownloadFile, downloadAllAsZip } = useFileTransfer()
const isIOSDevice = isIOS()

const overallPercent = computed(() => {
  if (props.session.total_size === 0) return 0
  return Math.round((props.session.transferred / props.session.total_size) * 100)
})

const progressStatus = computed(() => {
  switch (props.session.state) {
    case 'completed':
      return 'success'
    case 'failed':
      return 'exception'
    default:
      return undefined
  }
})

const stateText = computed(() => {
  switch (props.session.state) {
    case 'pending':
      return '等待确认'
    case 'accepted':
      return '已确认'
    case 'transferring':
      return '传输中'
    case 'completed':
      return '已完成'
    case 'partially_completed':
      return '部分完成'
    case 'failed':
      return '失败'
    default:
      return props.session.state
  }
})

function filePercent(file: FileInfo): number {
  if (file.size === 0) return 0
  return Math.round((file.transfer_size / file.size) * 100)
}

function fileStatus(status: string): '' | 'success' | 'exception' | 'warning' {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'exception'
    default:
      return ''
  }
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function downloadFile(file: FileInfo) {
  handleDownloadFile(props.session.id, { id: file.id, name: file.name })
}

function downloadAll() {
  // iPad: 单文件逐个预览（zip 解压困难）
  // PC: zip 批量下载
  if (isIOSDevice) {
    // iPad 上逐个打开可预览文件
    const successFiles = props.session.files.filter(f => f.status === 'success')
    if (successFiles.length === 1) {
      downloadFile(successFiles[0])
    } else {
      // 多文件时提示用户逐个下载
      alert('iPad 上请逐个点击文件旁边的"下载"按钮')
    }
  } else {
    // PC: zip 批量下载
    downloadAllAsZip(props.session.id)
  }
}
</script>

<style lang="scss" scoped>
.transfer-queue {
  background: white;
  border-radius: 8px;
  padding: 16px;
}

.queue-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;

  span {
    font-weight: bold;
    color: #303133;
  }
}

.overall-progress {
  margin-bottom: 16px;

  .progress-info {
    display: flex;
    justify-content: space-between;
    margin-top: 8px;
    color: #606266;
  }
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 8px;

  @media (max-width: 768px) {
    flex-wrap: wrap;
  }

  .file-name {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 150px;

    @media (max-width: 768px) {
      width: 100%;
    }
  }

  .file-progress {
    width: 150px;

    @media (max-width: 768px) {
      width: calc(100% - 80px);
    }
  }

  .file-size {
    color: #909399;
    min-width: 80px;
  }
}
</style>