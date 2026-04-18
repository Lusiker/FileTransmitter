<template>
  <div class="receiver-list">
    <div v-if="receivers.length === 0" class="empty-state">
      <el-icon size="48"><Loading /></el-icon>
      <p>正在搜索接收者...</p>
    </div>

    <div v-else class="receiver-cards">
      <div
        v-for="receiver in receivers"
        :key="receiver.id"
        class="receiver-card"
        :class="{ selected: selectedReceiver?.id === receiver.id }"
        @click="selectReceiver(receiver)"
      >
        <div class="card-header">
          <el-icon size="28" class="device-icon"><Monitor /></el-icon>
          <div class="device-info">
            <div class="device-name">{{ receiver.name || '接收者' }}</div>
            <div class="device-ip">{{ receiver.ip || '' }}</div>
          </div>
        </div>
        <div class="card-body">
          <div class="device-tags">
            <el-tag size="small" type="success">接收者</el-tag>
            <el-tag size="small" type="success">在线</el-tag>
          </div>
        </div>
        <div v-if="selectedReceiver?.id === receiver.id" class="selected-badge">
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
  receivers: Device[]
  selectedReceiver: Device | null
}>()

const emit = defineEmits<{
  'update:selectedReceiver': [receiver: Device | null]
}>()

function selectReceiver(receiver: Device) {
  emit('update:selectedReceiver', receiver)
}
</script>

<style lang="scss" scoped>
.receiver-list {
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

  .el-icon {
    color: #409EFF;
  }

  p {
    margin-top: 16px;
  }
}

.receiver-cards {
  display: grid;
  gap: 12px;
  grid-template-columns: 1fr;

  @media (min-width: 500px) {
    grid-template-columns: repeat(2, 1fr);
  }

  @media (min-width: 800px) {
    grid-template-columns: repeat(3, 1fr);
  }
}

.receiver-card {
  padding: 16px;
  border: 2px solid #EBEEF5;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  position: relative;

  &:hover {
    border-color: #409EFF;
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
      color: #409EFF;
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