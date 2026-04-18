<template>
  <div class="file-selector">
    <!-- Mobile warning -->
    <el-alert
      v-if="isMobile && fileList.length > 0 && fileList.length < 20"
      type="warning"
      :closable="false"
      style="margin-bottom: 12px"
    >
      移动端浏览器可能限制单次选择的文件数量，如需选择更多文件请点击"添加更多文件"
    </el-alert>

    <el-upload
      ref="uploadRef"
      v-model:file-list="fileList"
      :auto-upload="false"
      :multiple="true"
      :show-file-list="false"
      drag
      @change="handleChange"
    >
      <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
      <div class="el-upload__text">
        拖拽文件到此处或 <em>点击选择</em>
      </div>
      <template #tip>
        <div class="el-upload__tip">
          支持批量选择多个文件
        </div>
      </template>
    </el-upload>

    <!-- Add more files button (mobile friendly) -->
    <el-button
      v-if="fileList.length > 0"
      type="primary"
      size="small"
      style="margin-top: 12px"
      @click="triggerAddMore"
    >
      <el-icon><Plus /></el-icon>
      添加更多文件
    </el-button>

    <div v-if="fileList.length > 0" class="selected-files">
      <h3>已选择 {{ fileList.length }} 个文件</h3>
      <div class="total-size">
        总大小: {{ formatSize(totalSize) }}
      </div>

      <!-- Compact file list -->
      <div class="file-list-preview">
        <div
          v-for="file in fileList.slice(0, 5)"
          :key="file.name"
          class="file-item"
        >
          <span class="file-name">{{ file.name }}</span>
          <span class="file-size">{{ formatSize(file.size || 0) }}</span>
          <el-button
            type="danger"
            size="small"
            text
            @click="removeFile(file)"
          >
            <el-icon><Close /></el-icon>
          </el-button>
        </div>
        <div v-if="fileList.length > 5" class="more-files">
          还有 {{ fileList.length - 5 }} 个文件...
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { UploadUserFile } from 'element-plus'
import { Plus, Close } from '@element-plus/icons-vue'

defineProps<{
  modelValue: File[]
}>()

const emit = defineEmits<{
  change: [files: File[]]
}>()

const uploadRef = ref()
const fileList = ref<UploadUserFile[]>([])

// Detect mobile browser
const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent)

const totalSize = computed(() => {
  return fileList.value.reduce((sum, file) => {
    return sum + (file.size || 0)
  }, 0)
})

watch(fileList, (newList) => {
  console.log('[FileSelector] fileList changed, count:', newList.length)
  const files = newList.map(f => f.raw as File)
  console.log('[FileSelector] emitting files, count:', files.length)
  emit('change', files)
}, { deep: true })

function handleChange(file: UploadUserFile) {
  console.log('[FileSelector] File added:', file.name, 'total files:', fileList.value.length)
}

function triggerAddMore() {
  // Trigger the upload component to open file selector again
  // Files will be appended to existing fileList
  const inputEl = uploadRef.value?.$el?.querySelector('input[type="file"]')
  if (inputEl) {
    inputEl.click()
  }
}

function removeFile(file: UploadUserFile) {
  const index = fileList.value.indexOf(file)
  if (index > -1) {
    fileList.value.splice(index, 1)
  }
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}
</script>

<style lang="scss" scoped>
.file-selector {
  background: white;
  border-radius: 8px;
  padding: 16px;

  :deep(.el-upload-dragger) {
    width: 100%;
  }
}

.selected-files {
  margin-top: 16px;
  padding: 12px;
  background: #f0f9ff;
  border-radius: 8px;

  h3 {
    color: #303133;
    margin-bottom: 8px;
  }

  .total-size {
    color: #409EFF;
    font-weight: bold;
    margin-bottom: 12px;
  }
}

.file-list-preview {
  .file-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 0;
    border-bottom: 1px solid #ebeef5;

    .file-name {
      flex: 1;
      color: #303133;
      font-size: 0.85rem;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .file-size {
      color: #909399;
      font-size: 0.75rem;
    }
  }

  .more-files {
    padding: 8px 0;
    color: #909399;
    font-size: 0.85rem;
  }
}
</style>