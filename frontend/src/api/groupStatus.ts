/**
 * 分组状态页 API（面向所有登录用户）。
 *
 * 返回当前用户可用分组的 24h 状态：
 * - decode 速度 / TTFT：按 fast、normal 两种模式分别给出 EMA 衰减加权均值
 * - uptime（成功调用率）
 * - 缓存命中率（与渠道监控同口径：cache_read / (input + cache_creation + cache_read)）
 */

import { apiClient } from './client'

export interface GroupStatusTierStats {
  requests: number
  /** 平均 decode 速度（tokens/s），EMA 衰减加权；null 表示窗口内无可用样本 */
  decode_speed_tps: number | null
  /** 平均首 token 延迟（毫秒），EMA 衰减加权 */
  ttft_ms: number | null
  /** 平均缓存命中率（0-1），EMA 衰减加权 */
  cache_rate: number | null
}

export interface GroupStatusUptime {
  success_requests: number
  /** 窗口内错误总数（含业务限制） */
  error_requests: number
  /** 非业务限制错误数（余额/配额等用户侧原因不计入 uptime） */
  service_errors: number
  /** 成功调用率（0-1）；窗口内无任何调用时为 null */
  success_rate: number | null
}

export type GroupHealthStatus = 'healthy' | 'degraded' | 'down' | 'idle'

export interface GroupStatusEntry {
  group_id: number
  name: string
  platform: string
  description: string
  has_traffic: boolean
  status: GroupHealthStatus
  fast: GroupStatusTierStats
  normal: GroupStatusTierStats
  overall: GroupStatusTierStats
  uptime: GroupStatusUptime
}

export interface GroupStatusReport {
  generated_at: string
  window_hours: number
  ema_half_life_hours: number
  groups: GroupStatusEntry[]
}

// apiClient 的响应拦截器已把 { code, message, data } 解包成 data，
// 这里直接取 axios 的 response.data 即为业务负载。
export const groupStatusApi = {
  async getReport(): Promise<GroupStatusReport> {
    const { data } = await apiClient.get<GroupStatusReport>('/groups/status', { timeout: 15000 })
    return data
  },
}
