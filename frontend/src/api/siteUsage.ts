/**
 * 全站用量（脱敏只读视图）API
 *
 * 面向所有登录用户：口径与管理端 Usage 页一致，但用户名 / Key 名已在
 * 服务端脱敏（保留前 2 后 2 字符，中间以 ** 代替），且不含 IP /
 * User-Agent / 上游账号等管理员字段。
 */

import { apiClient } from './client'
import type {
  EndpointStat,
  GroupStat,
  ModelStat,
  PaginatedResponse,
  TrendDataPoint,
  UsageRequestType
} from '@/types'

export interface SiteUsageQueryParams {
  page?: number
  page_size?: number
  start_date?: string
  end_date?: string
  group_id?: number | null
  model?: string | null
  request_type?: UsageRequestType | null
  stream?: boolean
  native_compaction_v2?: boolean | null
  billing_type?: number | null
  billing_mode?: string | null
  sort_by?: string
  sort_order?: 'asc' | 'desc'
  exact_total?: boolean
}

export interface SiteUsageUser {
  id: number
  /** 已脱敏（如 ra**14） */
  email: string
}

export interface SiteUsageApiKey {
  id: number
  /** 已脱敏 */
  name: string
}

export interface SiteUsageGroup {
  id: number
  name: string
  platform: string
}

export interface SiteUsageLog {
  id: number
  created_at: string
  user_id: number
  user?: SiteUsageUser | null
  api_key_id: number
  api_key?: SiteUsageApiKey | null
  group_id?: number | null
  group?: SiteUsageGroup | null
  request_id: string
  model: string
  service_tier?: string | null
  reasoning_effort?: string | null
  inbound_endpoint?: string | null
  request_type: string
  stream: boolean
  billing_type: number
  billing_mode?: string | null
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  input_cost: number
  output_cost: number
  cache_creation_cost: number
  cache_read_cost: number
  total_cost: number
  actual_cost: number
  rate_multiplier: number
  duration_ms?: number | null
  first_token_ms?: number | null
  image_count: number
  video_count: number
}

export interface SiteUsageStats {
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cache_tokens: number
  total_cache_creation_tokens: number
  total_cache_read_tokens: number
  total_tokens: number
  total_cost: number
  total_actual_cost: number
  average_duration_ms: number
  endpoints?: EndpointStat[]
}

export interface SiteUsageSnapshotV2Response {
  generated_at: string
  start_date: string
  end_date: string
  granularity: string
  trend?: TrendDataPoint[]
  groups?: GroupStat[]
}

export interface SiteUsageRankingItem {
  user_id: number
  /** 已脱敏 */
  email: string
  requests: number
  input_tokens: number
  output_tokens: number
  cache_tokens: number
  total_tokens: number
  cost: number
  actual_cost: number
}

export async function list(
  params: SiteUsageQueryParams,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<SiteUsageLog>> {
  const { data } = await apiClient.get<PaginatedResponse<SiteUsageLog>>('/usage/site', {
    params,
    signal: options?.signal
  })
  return data
}

export async function getStats(params: SiteUsageQueryParams): Promise<SiteUsageStats> {
  const { data } = await apiClient.get<SiteUsageStats>('/usage/site/stats', { params })
  return data
}

export async function getSnapshotV2(
  params: SiteUsageQueryParams & { granularity?: 'day' | 'hour' }
): Promise<SiteUsageSnapshotV2Response> {
  const { data } = await apiClient.get<SiteUsageSnapshotV2Response>('/usage/site/dashboard/snapshot-v2', {
    params
  })
  return data
}

export async function getModelStats(
  params: SiteUsageQueryParams
): Promise<{ models: ModelStat[] }> {
  const { data } = await apiClient.get<{ models: ModelStat[] }>('/usage/site/dashboard/model-stats', {
    params
  })
  return data
}

export async function getRanking(
  params: SiteUsageQueryParams & { limit?: number }
): Promise<SiteUsageRankingItem[]> {
  const { data } = await apiClient.get<SiteUsageRankingItem[]>('/usage/site/ranking', { params })
  return data
}

export const siteUsageAPI = {
  list,
  getStats,
  getSnapshotV2,
  getModelStats,
  getRanking
}

export default siteUsageAPI
