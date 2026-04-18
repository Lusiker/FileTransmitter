<template>
  <div class="failed-files">
    <el-alert type="error" :closable="false">
      <template #title>
        有 {{ files.length }} 个文件传输失败
      </template>
    </el-alert>

    <div class="failed-list">
      <el-checkbox-group v-model="selectedFileIds">
        <div
          v-for="file in files"
          :key="file.id"
          class="failed-item"
        >
          <el-checkbox :value="file.id">
            <div class="file-info">
              <span class="name">{{ file.name }}</span>
              <span class="error">{{ file.error }}</span>
            </div>
          </el-checkbox>
          <el-tag type="danger" size="small">失败</el-tag>
        </div>
      </el-checkbox-group>
    </div>

    <div class="actions">
      <el-button
        type="primary"
        :disabled="selectedFileIds.length === 0"
        @click="handleRetry"
      >
        重试选中文件
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { FileInfo } from '@/types'

defineProps<{
  files: FileInfo[]
  sessionId?: string
}>()

const emit = defineEmits<{
  retry: [fileIds: string[]]
}>()

const selectedFileIds = ref<string[]>([])

function handleRetry() {
  emit('retry', selectedFileIds.value)
  selectedFileIds.value = []
}
</script>

<style lang="scss" scoped>
.failed-files {
  background: white;
  border-radius: 8px;
  padding: 16px;
}

.failed-list {
  margin-top: 16px;
}

.failed-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  margin-bottom: 8px;
  background: #fef0f0;
  border-radius: 8px;

  .file-info {
    .name {
      color: #303133;
    }

    .error {
      color: #f56c6c;
      margin-left: 8px;
      font-size: 0.875rem;
    }
  }
}

.actions {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>