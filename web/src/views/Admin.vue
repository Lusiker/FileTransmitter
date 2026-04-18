<template>
  <div class="admin-page">
    <header class="page-header">
      <div class="header-row">
        <el-button @click="goBack" circle size="small">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <h1>管理员监控</h1>
      </div>
      <div class="header-actions">
        <el-button type="danger" size="small" @click="cleanCompleted">
          清理已完成
        </el-button>
        <el-button size="small" @click="refreshAll">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </header>

    <main class="page-main">
      <!-- 系统状态概览 -->
      <section class="status-section">
        <el-row :gutter="16">
          <el-col :span="6">
            <el-card shadow="hover" class="status-card">
              <div class="status-value">{{ status.devices?.total || 0 }}</div>
              <div class="status-label">在线设备</div>
              <div class="status-detail">
                发送: {{ status.devices?.senders || 0 }} / 接收: {{ status.devices?.receivers || 0 }}
              </div>
            </el-card>
          </el-col>
          <el-col :span="6">
            <el-card shadow="hover" class="status-card">
              <div class="status-value">{{ status.sessions?.active || 0 }}</div>
              <div class="status-label">活跃任务</div>
              <div class="status-detail">
                完成: {{ status.sessions?.completed || 0 }} / 失败: {{ status.sessions?.failed || 0 }}
              </div>
            </el-card>
          </el-col>
          <el-col :span="6">
            <el-card shadow="hover" class="status-card storage-card">
              <div class="status-value">{{ formatSize(status.storage?.used_bytes || 0) }}</div>
              <div class="status-label">存储占用</div>
              <div class="status-detail">{{ status.storage?.temp_dir || './tmp' }}</div>
            </el-card>
          </el-col>
          <el-col :span="6">
            <el-card shadow="hover" class="status-card">
              <div class="status-value timestamp">{{ status.timestamp || '--' }}</div>
              <div class="status-label">更新时间</div>
            </el-card>
          </el-col>
        </el-row>
      </section>

      <!-- 设备连接 -->
      <section class="devices-section">
        <h2>设备连接状态</h2>
        <el-table :data="devices" stripe size="small" v-loading="loadingDevices">
          <el-table-column prop="name" label="名称" width="120" />
          <el-table-column prop="role" label="角色" width="100">
            <template #default="{ row }">
              <el-tag :type="row.role === 'sender' ? 'primary' : 'success'" size="small">
                {{ row.role === 'sender' ? '发送端' : '接收端' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="ip" label="IP地址" width="150" />
          <el-table-column prop="id" label="设备ID" show-overflow-tooltip />
          <el-table-column prop="last_seen" label="最后活动" width="180" />
        </el-table>
      </section>

      <!-- 任务状态 -->
      <section class="sessions-section">
        <h2>任务状态</h2>
        <el-table :data="sessions" stripe size="small" v-loading="loadingSessions"
                  @row-click="expandSession">
          <el-table-column type="expand">
            <template #default="{ row }">
              <div class="session-files">
                <h4>文件列表</h4>
                <el-table :data="row.files" size="small">
                  <el-table-column prop="name" label="文件名" show-overflow-tooltip />
                  <el-table-column prop="size" label="大小" width="120">
                    <template #default="{ row }">{{ formatSize(row.size) }}</template>
                  </el-table-column>
                  <el-table-column prop="status" label="状态" width="100">
                    <template #default="{ row }">
                      <el-tag :type="getStatusType(row.status)" size="small">
                        {{ getStatusLabel(row.status) }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column prop="transfer_size" label="已传输" width="120">
                    <template #default="{ row }">
                      {{ row.status === 'success' ? formatSize(row.size) : formatSize(row.transfer_size) }}
                    </template>
                  </el-table-column>
                </el-table>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="id" label="会话ID" width="280" show-overflow-tooltip />
          <el-table-column prop="state" label="状态" width="120">
            <template #default="{ row }">
              <el-tag :type="getStateType(row.state)" size="small">
                {{ getStateLabel(row.state) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="total_size" label="总大小" width="100">
            <template #default="{ row }">{{ formatSize(row.total_size) }}</template>
          </el-table-column>
          <el-table-column label="进度" width="100">
            <template #default="{ row }">
              <el-progress
                :percentage="Math.round((row.transferred / row.total_size) * 100) || 0"
                :status="row.state === 'completed' ? 'success' : undefined"
                :stroke-width="6"
              />
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="180" />
        </el-table>
      </section>

      <!-- 存储使用 -->
      <section class="storage-section">
        <h2>临时存储占用</h2>
        <el-alert
          v-if="storage.total_size_mb > 100"
          title="存储占用较高，建议清理已完成任务"
          type="warning"
          :closable="false"
          show-icon
          style="margin-bottom: 16px"
        />
        <el-table :data="storage.sessions" stripe size="small" v-loading="loadingStorage">
          <el-table-column prop="session_id" label="会话ID" width="280" show-overflow-tooltip />
          <el-table-column prop="size_mb" label="占用大小" width="120" sortable>
            <template #default="{ row }">
              <span :class="{ 'size-warning': row.size_mb > 50 }">
                {{ row.size_mb.toFixed(2) }} MB
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="files_count" label="文件数" width="80" />
          <el-table-column prop="state" label="会话状态" width="120">
            <template #default="{ row }">
              <el-tag :type="getStateType(row.state)" size="small">
                {{ getStateLabel(row.state) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="has_chunks" label="分片状态" width="100">
            <template #default="{ row }">
              <el-tag v-if="row.has_chunks" type="warning" size="small">有分片</el-tag>
              <el-tag v-else type="info" size="small">已合并</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="canClean(row.state)"
                type="danger"
                size="small"
                text
                @click="cleanSession(row.session_id)"
              >
                清理
              </el-button>
              <el-tag v-else type="info" size="small">传输中</el-tag>
            </template>
          </el-table-column>
        </el-table>

        <div class="storage-summary">
          <span>总计: {{ storage.total_count }} 个会话目录，占用 {{ storage.total_size_mb?.toFixed(2) || 0 }} MB</span>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Refresh } from '@element-plus/icons-vue'
import axios from 'axios'

const router = useRouter()

// 动态获取 API 地址
function getApiBase(): string {
  const host = window.location.hostname
  return `http://${host}:8080`
}

const API_BASE = getApiBase()

// 数据
const status = ref<any>({
  devices: { total: 0, senders: 0, receivers: 0 },
  sessions: { active: 0, completed: 0, failed: 0 },
  storage: { used_bytes: 0, used_mb: 0, temp_dir: '' },
  timestamp: ''
})
const devices = ref<any[]>([])
const sessions = ref<any[]>([])
const storage = ref<any>({
  sessions: [],
  total_count: 0,
  total_size_mb: 0,
  temp_dir: ''
})

// 加载状态
const loadingDevices = ref(false)
const loadingSessions = ref(false)
const loadingStorage = ref(false)

// 自动刷新定时器
let refreshTimer: number | null = null

// 格式化大小
function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 状态映射
function getStatusType(status: string): string {
  const map: Record<string, string> = {
    pending: 'info',
    transferring: 'primary',
    merging: 'warning',
    success: 'success',
    failed: 'danger'
  }
  return map[status] || 'info'
}

function getStatusLabel(status: string): string {
  const map: Record<string, string> = {
    pending: '等待',
    transferring: '传输中',
    merging: '合并中',
    success: '成功',
    failed: '失败'
  }
  return map[status] || status
}

function getStateType(state: string): string {
  const map: Record<string, string> = {
    pending: 'info',
    accepted: 'primary',
    transferring: 'primary',
    completed: 'success',
    partially_completed: 'warning',
    failed: 'danger',
    cancelled: 'info'
  }
  return map[state] || 'info'
}

function getStateLabel(state: string): string {
  const map: Record<string, string> = {
    pending: '等待确认',
    accepted: '已接受',
    transferring: '传输中',
    completed: '已完成',
    partially_completed: '部分完成',
    failed: '失败',
    cancelled: '已取消'
  }
  return map[state] || state
}

function canClean(state: string): boolean {
  // 只有非传输中的会话可以清理
  return state !== 'transferring' && state !== 'accepted'
}

// 刷新数据
async function refreshAll() {
  await Promise.all([
    fetchStatus(),
    fetchDevices(),
    fetchSessions(),
    fetchStorage()
  ])
}

async function fetchStatus() {
  try {
    const res = await axios.get(`${API_BASE}/api/v1/admin/status`)
    status.value = res.data
  } catch (error) {
    console.error('Failed to fetch status:', error)
  }
}

async function fetchDevices() {
  loadingDevices.value = true
  try {
    const res = await axios.get(`${API_BASE}/api/v1/admin/devices`)
    devices.value = res.data.devices || []
  } catch (error) {
    console.error('Failed to fetch devices:', error)
  }
  loadingDevices.value = false
}

async function fetchSessions() {
  loadingSessions.value = true
  try {
    const res = await axios.get(`${API_BASE}/api/v1/admin/sessions`)
    sessions.value = res.data.sessions || []
  } catch (error) {
    console.error('Failed to fetch sessions:', error)
  }
  loadingSessions.value = false
}

async function fetchStorage() {
  loadingStorage.value = true
  try {
    const res = await axios.get(`${API_BASE}/api/v1/admin/storage`)
    storage.value = res.data
  } catch (error) {
    console.error('Failed to fetch storage:', error)
  }
  loadingStorage.value = false
}

// 清理单个会话
async function cleanSession(sessionId: string) {
  try {
    await ElMessageBox.confirm(
      `确定要清理会话 ${sessionId} 的临时文件吗？`,
      '确认清理',
      { type: 'warning' }
    )

    const res = await axios.delete(`${API_BASE}/api/v1/admin/storage/${sessionId}`)
    ElMessage.success(`已清理 ${res.data.size_cleaned_mb.toFixed(2)} MB`)
    await fetchStorage()
    await fetchStatus()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || '清理失败')
    }
  }
}

// 清理所有已完成任务
async function cleanCompleted() {
  try {
    await ElMessageBox.confirm(
      '确定要清理所有已完成/取消/失败的任务临时文件吗？',
      '确认批量清理',
      { type: 'warning' }
    )

    const res = await axios.post(`${API_BASE}/api/v1/admin/storage/clean-completed`)
    ElMessage.success(`已清理 ${res.data.cleaned_count} 个会话，共 ${res.data.total_cleaned_mb.toFixed(2)} MB`)
    await refreshAll()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || '清理失败')
    }
  }
}

function expandSession(_row: any) {
  // 展开/折叠行（由 el-table 自动处理）
}

function goBack() {
  router.push('/')
}

onMounted(async () => {
  await refreshAll()

  // 每 5 秒自动刷新
  refreshTimer = window.setInterval(() => {
    refreshAll()
  }, 5000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<style lang="scss" scoped>
.admin-page {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: #f5f7fa;
}

.page-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 16px;
  background: white;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);

  .header-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  h1 {
    font-size: 1.25rem;
    color: #303133;
    margin: 0;
  }

  .header-actions {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
}

.page-main {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  min-height: 0;

  section {
    margin-bottom: 16px;

    h2 {
      color: #303133;
      margin-bottom: 12px;
      font-size: 1.1rem;
    }
  }
}

.status-card {
  text-align: center;
  min-height: 100px;

  .status-value {
    font-size: 1.5rem;
    font-weight: bold;
    color: #409EFF;
    line-height: 1.5;

    &.timestamp {
      font-size: 0.9rem;
      color: #909399;
    }
  }

  .status-label {
    color: #606266;
    margin-top: 8px;
  }

  .status-detail {
    color: #909399;
    font-size: 0.85rem;
    margin-top: 4px;
  }

  &.storage-card .status-value {
    color: #E6A23C;
  }
}

.session-files {
  padding: 16px;

  h4 {
    margin-bottom: 12px;
    color: #606266;
  }
}

.storage-summary {
  margin-top: 16px;
  color: #606266;
  font-size: 0.9rem;
}

.size-warning {
  color: #E6A23C;
  font-weight: bold;
}

@media (min-width: 600px) {
  .page-header {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    padding: 16px 24px;
    gap: 16px;

    h1 {
      font-size: 1.5rem;
    }

    .header-actions {
      flex-wrap: nowrap;
    }
  }

  .page-main {
    padding: 24px;

    section {
      margin-bottom: 24px;

      h2 {
        font-size: 1.25rem;
        margin-bottom: 16px;
      }
    }
  }
}
</style>