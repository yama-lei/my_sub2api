/**
 * 分组状态页 API（面向所有登录用户）。
 *
 * 返回当前用户可用分组的 24h 状态：
 * - 分组级 + 模型级（每个模型独立展示）的 decode 速度 / TTFT（按 fast、normal 两种
 *   模式分别给出 EMA 衰减加权均值）、uptime（成功调用率）、缓存命中率
 * - 每个模型附完整时间桶网格的序列（曲线图与 uptime 条的数据源）
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

/** 模型时间序列的单个桶（无流量桶的指标为 null） */
export interface GroupModelSeriesPoint {
  bucket_start: string
  requests: number
  service_errors: number
  /** 桶内成功调用率（uptime），无调用时为 null */
  success_rate: number | null
  decode_speed_tps: number | null
  ttft_ms: number | null
  cache_rate: number | null
}

/** 单个模型的状态（与分组同一套 EMA 口径 + 时间序列） */
export interface GroupModelStatus {
  model: string
  fast: GroupStatusTierStats
  normal: GroupStatusTierStats
  overall: GroupStatusTierStats
  /** 完整时间桶网格（含无流量桶），按 bucket_start 升序 */
  series: GroupModelSeriesPoint[]
}

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
  /** 窗口内有流量的模型（按调用量降序） */
  models: GroupModelStatus[]
}

export interface GroupStatusReport {
  generated_at: string
  window_hours: number
  ema_half_life_hours: number
  /** 模型时间序列的桶宽（分钟） */
  series_bucket_minutes: number
  groups: GroupStatusEntry[]
}

// apiClient 的响应拦截器已把 { code, message, data } 解包成 data，
// 这里直接取 axios 的 response.data 即为业务负载。
export const groupStatusApi = {
  async getReport(): Promise<GroupStatusReport> {
    const { data } = await apiClient.get<GroupStatusReport>('/groups/status', { timeout: 20000 })
    return data
  },
}
