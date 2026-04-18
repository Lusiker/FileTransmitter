<template>
  <div class="save-location">
    <!-- Current save path display -->
    <div class="current-path">
      <el-input
        v-model="savePath"
        placeholder="保存路径"
      >
        <template #prepend>
          <el-icon><Folder /></el-icon>
        </template>
      </el-input>
    </div>

    <!-- Path selection - different on mobile -->
    <div class="path-selection">
      <!-- Desktop: show folder picker button -->
      <div v-if="!isMobile" class="path-buttons">
        <el-button type="primary" size="small" @click="selectFolder">
          <el-icon><FolderOpened /></el-icon>
          选择目录
        </el-button>
      </div>

      <!-- Quick paths - horizontal layout -->
      <div class="quick-paths">
        <el-tag
          v-for="path in quickPaths"
          :key="path.value"
          :type="savePath === path.value ? 'success' : 'info'"
          :effect="savePath === path.value ? 'dark' : 'plain'"
          class="path-tag"
          @click="setPath(path.value)"
        >
          {{ path.label }}
        </el-tag>
      </div>

      <!-- Custom subdirectory name -->
      <div class="custom-folder">
        <p>自定义文件夹名:</p>
        <el-input
          v-model="customFolderName"
          placeholder="如: 我的下载"
          size="small"
          @keyup.enter="applyCustomFolder"
        />
        <el-button type="primary" size="small" @click="applyCustomFolder">
          应用
        </el-button>
      </div>
    </div>

    <!-- Hint -->
    <p class="hint">文件将保存到服务端的 {{ savePath }} 目录</p>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'

const props = defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  'update:modelValue': [path: string]
}>()

const savePath = ref(props.modelValue || './downloads')
const customFolderName = ref('')
const isMobile = ref(false)

// Quick path options
const quickPaths = [
  { label: 'Downloads', value: './downloads' },
  { label: 'Documents', value: './documents' },
  { label: '共享目录', value: './shared' },
]

onMounted(() => {
  // Detect mobile device
  isMobile.value = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent)
})

watch(savePath, (newPath) => {
  emit('update:modelValue', newPath)
})

watch(() => props.modelValue, (newVal) => {
  savePath.value = newVal
})

function setPath(path: string) {
  savePath.value = path
}

function applyCustomFolder() {
  if (customFolderName.value.trim()) {
    // 在 downloads 下创建自定义子目录
    savePath.value = `./downloads/${customFolderName.value.trim()}`
    customFolderName.value = ''
  }
}

// Folder picker - only works on desktop browsers
async function selectFolder() {
  if ('showDirectoryPicker' in window) {
    try {
      const dirHandle = await (window as any).showDirectoryPicker()
      savePath.value = dirHandle.name || './downloads'
    } catch (e) {
      console.log('Directory picker cancelled')
    }
  } else {
    alert('当前浏览器不支持目录选择')
  }
}
</script>

<style lang="scss" scoped>
.save-location {
  background: white;
  border-radius: 8px;
  padding: 16px;
}

.current-path {
  margin-bottom: 12px;
}

.path-selection {
  .path-buttons {
    margin-bottom: 12px;
  }

  .quick-paths {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 16px;

    .path-tag {
      cursor: pointer;
      transition: all 0.2s;

      &:hover {
        transform: scale(1.02);
      }
    }
  }

  .custom-folder {
    p {
      color: #606266;
      font-size: 0.85rem;
      margin-bottom: 8px;
    }

    display: flex;
    gap: 8px;
    align-items: center;

    .el-input {
      flex: 1;
    }
  }
}

.hint {
  color: #909399;
  font-size: 0.85rem;
  margin-top: 12px;
}
</style>