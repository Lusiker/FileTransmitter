<template>
  <el-tag
    :type="tagType"
    size="small"
    effect="light"
  >
    <el-icon v-if="status === 'success'"><Check /></el-icon>
    <el-icon v-if="status === 'failed'"><Close /></el-icon>
    <el-icon v-if="status === 'transferring' || status === 'merging'"><Loading /></el-icon>
    {{ statusText }}
  </el-tag>
  <span v-if="error" class="error-text">{{ error }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { FileStatus } from '@/types'

const props = defineProps<{
  status: FileStatus
  error?: string
}>()

const tagType = computed(() => {
  switch (props.status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'transferring':
      return ''
    case 'merging':
      return 'warning'
    case 'pending':
      return 'info'
    default:
      return 'info'
  }
})

const statusText = computed(() => {
  switch (props.status) {
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    case 'transferring':
      return '传输中'
    case 'merging':
      return '处理中'
    case 'pending':
      return '等待'
    default:
      return props.status
  }
})
</script>

<style lang="scss" scoped>
.error-text {
  color: #f56c6c;
  font-size: 0.75rem;
  margin-left: 8px;
}
</style>