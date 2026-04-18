<template>
  <div class="sender-discovery">
    <div v-if="senders.length === 0" class="empty-state">
      <el-icon class="loading-icon" size="48"><Loading /></el-icon>
      <p>正在搜索发送者...</p>
      <el-button type="primary" size="small" @click="$emit('refresh')">
        刷新
      </el-button>
    </div>

    <div v-else class="sender-cards">
      <div
        v-for="sender in senders"
        :key="sender.id"
        class="sender-card"
        :class="{ selected: selectedSender?.id === sender.id }"
        @click="selectSender(sender)"
      >
        <div class="card-header">
          <el-icon size="28" class="device-icon"><Monitor /></el-icon>
          <div class="device-info">
            <div class="device-name">{{ sender.name || '发送者' }}</div>
            <div class="device-ip">{{ sender.ip || '' }}</div>
          </div>
        </div>
        <div class="card-body">
          <div class="device-tags">
            <el-tag size="small" type="primary">发送者</el-tag>
            <el-tag size="small" type="success">在线</el-tag>
          </div>
        </div>
        <div v-if="selectedSender?.id === sender.id" class="selected-badge">
          <el-icon><Check /></el-icon>
          已选择
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Device } from '@/types'

defineProps<{
  senders: Device[]
  selectedSender: Device | null
}>()

const emit = defineEmits<{
  'update:selectedSender': [sender: Device | null]
  refresh: []
}>()

function selectSender(sender: Device) {
  emit('update:selectedSender', sender)
}
</script>

<style lang="scss" scoped>
.sender-discovery {
  background: white;
  border-radius: 8px;
  padding: 16px;
  min-height: 120px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 32px;
  color: #909399;

  .loading-icon {
    animation: spin 1s linear infinite;
    color: #67C23A;
  }

  p {
    margin: 16px 0;
  }
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.sender-cards {
  display: grid;
  gap: 12px;
  grid-template-columns: 1fr;

  @media (min-width: 500px) {
    grid-template-columns: repeat(2, 1fr);
  }
}

.sender-card {
  padding: 16px;
  border: 2px solid #EBEEF5;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  position: relative;

  &:hover {
    border-color: #67C23A;
    background: #f9f9f9;
  }

  &.selected {
    border-color: #67C23A;
    background: #f0f9eb;

    .selected-badge {
      display: flex;
    }
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;

    .device-icon {
      color: #67C23A;
    }

    .device-info {
      .device-name {
        font-size: 1rem;
        font-weight: 600;
        color: #303133;
      }
      .device-ip {
        font-size: 0.75rem;
        color: #909399;
      }
    }
  }

  .card-body {
    .device-tags {
      display: flex;
      gap: 8px;
    }
  }

  .selected-badge {
    position: absolute;
    top: 8px;
    right: 8px;
    display: none;
    align-items: center;
    gap: 4px;
    color: #67C23A;
    font-size: 0.85rem;
    background: #e1f3d8;
    padding: 4px 8px;
    border-radius: 4px;
  }
}
</style>