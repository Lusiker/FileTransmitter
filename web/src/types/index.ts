// Device types
export interface Device {
  id: string
  name: string
  role: 'sender' | 'receiver'
  ip?: string
  http_port?: number
  last_seen?: string
  is_online?: boolean
}

// WebSocket device status data (matches backend)
export interface DeviceStatusData {
  device_id: string
  device_name: string
  role: 'sender' | 'receiver'
  ip?: string
  http_port?: number
  is_online: boolean
}

// Convert DeviceStatusData to Device
export function toDevice(data: DeviceStatusData): Device {
  return {
    id: data.device_id,
    name: data.device_name,
    role: data.role,
    ip: data.ip,
    http_port: data.http_port,
    is_online: data.is_online
  }
}

// Session types
export type SessionState =
  | 'pending'
  | 'accepted'
  | 'transferring'
  | 'completed'
  | 'partially_completed'
  | 'cancelled'
  | 'failed'

export type FileStatus =
  | 'pending'
  | 'transferring'
  | 'merging'
  | 'success'
  | 'failed'

export interface FileInfo {
  id: string
  name: string
  size: number
  mime_type: string
  hash: string
  status: FileStatus
  error?: string
  transfer_size: number
}

export interface Session {
  id: string
  sender_id: string
  receiver_id: string
  state: SessionState
  files: FileInfo[]
  created_at: string
  updated_at: string
  total_size: number
  transferred: number
  save_path: string
}

// WebSocket message types
export type WSMessageType =
  | 'device_online'
  | 'device_offline'
  | 'session_created'
  | 'session_accepted'
  | 'transfer_progress'
  | 'file_complete'
  | 'file_failed'
  | 'transfer_complete'
  | 'transfer_failed'
  | 'error'

export interface WSMessage<T = any> {
  type: WSMessageType
  data: T
}

export interface TransferProgressData {
  session_id: string
  file_id: string
  bytes: number
  total: number
  percent: number
}

export interface FileStatusData {
  session_id: string
  file_id: string
  status: FileStatus
  error?: string
}

// API request types
export interface CreateSessionRequest {
  sender_id: string
  receiver_id: string
  files: {
    name: string
    size: number
    mime_type?: string
    hash?: string
  }[]
}

export interface AcceptSessionRequest {
  save_path: string
}

export interface RetryFilesRequest {
  file_ids: string[]
}

// App state
export type AppRole = 'sender' | 'receiver' | null