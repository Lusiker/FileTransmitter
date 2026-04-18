<template>
  <div class="dashboard-page">
    <header class="dashboard-header">
      <h1>FileTransmitter</h1>
      <p class="subtitle">局域网文件传输</p>
      <el-button
        type="info"
        size="small"
        text
        @click="goAdmin"
        class="admin-link"
      >
        <el-icon><Setting /></el-icon>
        管理员
      </el-button>
    </header>

    <main class="dashboard-main">
      <!-- Device Info Card -->
      <section class="device-card">
        <div class="device-info-row">
          <span class="label">设备名称</span>
          <el-input
            v-model="deviceName"
            size="small"
            placeholder="输入名称"
            style="width: 160px"
          >
            <template #append>
              <el-button size="small" @click="saveName">保存</el-button>
            </template>
          </el-input>
        </div>
        <div class="device-info-row">
          <span class="label">IP 地址</span>
          <span class="value">{{ deviceStore.deviceIP || '获取中...' }}</span>
        </div>
        <div class="device-info-row">
          <span class="label">连接状态</span>
          <el-tag :type="deviceStore.isConnected ? 'success' : 'danger'" size="small">
            {{ deviceStore.isConnected ? '在线' : '离线' }}
          </el-tag>
        </div>
      </section>

      <!-- Role Selection -->
      <section class="role-selection">
        <h3>选择角色</h3>
        <div class="role-cards">
          <div
            class="role-card sender"
            @click="selectRole('sender')"
          >
            <el-icon size="32"><Upload /></el-icon>
            <div class="role-info">
              <h4>发送文件</h4>
              <p>选择文件并发送</p>
            </div>
          </div>

          <div
            class="role-card receiver"
            @click="selectRole('receiver')"
          >
            <el-icon size="32"><Download /></el-icon>
            <div class="role-info">
              <h4>接收文件</h4>
              <p>发现发送者并接收</p>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="dashboard-footer">
      <p>设备将自动发现同一局域网内的其他设备</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useDeviceStore } from '@/stores/device'
import { Upload, Download, Setting } from '@element-plus/icons-vue'

const router = useRouter()
const deviceStore = useDeviceStore()

const deviceName = ref('')

onMounted(() => {
  // 立即获取当前值
  deviceName.value = deviceStore.deviceName
  console.log('[Dashboard] Device name:', deviceStore.deviceName, 'IP:', deviceStore.deviceIP)
})

// 监听 deviceStore.deviceName 变化（initDevice 完成后）
watch(() => deviceStore.deviceName, (newName) => {
  if (newName && !deviceName.value) {
    deviceName.value = newName
  }
})

function saveName() {
  if (deviceName.value.trim()) {
    deviceStore.setName(deviceName.value.trim())
  }
}

function selectRole(role: 'sender' | 'receiver') {
  saveName()
  deviceStore.setRole(role)
  // 等待 WebSocket 连接完成后再跳转
  setTimeout(() => {
    router.push(`/${role}`)
  }, 100)
}

function goAdmin() {
  router.push('/admin')
}
</script>

<style lang="scss" scoped>
.dashboard-page {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
  overflow-y: auto;
}

.dashboard-header {
  text-align: center;
  padding: 16px 20px;
  position: relative;

  h1 {
    font-size: 1.5rem;
    color: #303133;
    margin-bottom: 4px;
  }

  .subtitle {
    color: #606266;
    font-size: 0.85rem;
  }

  .admin-link {
    position: absolute;
    top: 16px;
    right: 20px;
  }
}

.dashboard-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px 20px;
  max-width: 500px;
  margin: 0 auto;
  width: 100%;

  section {
    background: white;
    border-radius: 12px;
    padding: 16px;

    h3 {
      color: #303133;
      margin-bottom: 12px;
      font-size: 1rem;
    }
  }
}

.device-card {
  .device-info-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 0;

    &:not(:last-child) {
      border-bottom: 1px solid #EBEEF5;
    }

    .label {
      color: #606266;
      font-size: 0.9rem;
    }

    .value {
      color: #303133;
      font-size: 0.9rem;
    }
  }
}

.role-selection {
  .role-cards {
    display: flex;
    gap: 12px;
  }

  .role-card {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 16px 12px;
    border-radius: 12px;
    cursor: pointer;
    transition: all 0.2s;

    &.sender {
      background: #ecf5ff;
      border: 2px solid #409EFF;

      &:hover {
        background: #d9ecff;
      }

      .el-icon {
        color: #409EFF;
      }
    }

    &.receiver {
      background: #f0f9eb;
      border: 2px solid #67C23A;

      &:hover {
        background: #e1f3d8;
      }

      .el-icon {
        color: #67C23A;
      }
    }

    .role-info {
      text-align: center;
      margin-top: 10px;

      h4 {
        color: #303133;
        margin-bottom: 4px;
        font-size: 0.9rem;
      }

      p {
        color: #909399;
        font-size: 0.75rem;
      }
    }
  }
}

.dashboard-footer {
  text-align: center;
  padding: 12px 20px;
  color: #909399;
  font-size: 0.8rem;
}
</style>