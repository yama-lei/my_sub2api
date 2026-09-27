/**
 * Admin ModelTrace API endpoints
 * 模型真假检测（ModelTrace 指纹归因）
 */

import apiClient, { buildApiUrl } from '../client'
import { ADMIN_UI_REQUEST_HEADER } from '../adminUIRequest'

/** 指纹库支持的模型选项 */
export interface ModelTraceModelOption {
  id: string
  display_name: string
  family: string
  family_name: string
}

/** 可用于检测的账号（key）选项 */
export interface ModelTraceAccountOption {
  id: number
  name: string
  platform: string
  type: string
  status: string
  credential_type: string
}

/** 指纹库概况 */
export interface ModelTraceBankInfo {
  built_at: string
  model_count: number
  recommended_queries: number
  minimum_valid_numbers: number
}

export interface ModelTraceOptionsResponse {
  models: ModelTraceModelOption[]
  accounts: ModelTraceAccountOption[]
  bank_info: ModelTraceBankInfo
}

/** 单个候选模型的归因结果 */
export interface ModelTraceModelResult {
  model: string
  display_name: string
  family: string
  family_name: string
  probability: number
  conditional_probability: number
  profile_similarity: number
  score: number
}

/** 检测诊断（单份回答） */
export interface ModelTraceDiagnostic {
  index: number
  parsed_numbers: number
  minimum_numbers: number
  accepted: boolean
}

/** 家族概率 */
export interface ModelTraceFamilyResult {
  family: string
  display_name: string
  probability: number
}

/** 完整归因结果（后端 modeltrace.Attribution） */
export interface ModelTraceAttribution {
  prediction: string
  prediction_name: string
  probability: number
  used_outputs: number
  results: ModelTraceModelResult[]
  diagnostics: ModelTraceDiagnostic[]
  calibration_beta: number
  calibration_cv_accuracy: number
  calibration_queries: number
  family_prediction: string
  family_prediction_name: string
  family_probability: number
  family_probabilities: ModelTraceFamilyResult[]
  method: string
}

/** 检测结论 */
export interface ModelTraceVerdict {
  status: 'match' | 'family_mismatch' | 'mismatch'
  confidence: 'high' | 'medium' | 'low'
  claimed_model: string
  predicted_model: string
  claimed_probability: number
}

/** 单份回答采集报告 */
export interface ModelTraceResponseReport {
  challenge_id: string
  expected_count: number
  parsed_numbers: number
  accepted: boolean
  excerpt: string
}

/** 最终检测负载（SSE result 事件 data 字段） */
export interface ModelTraceResult {
  account: ModelTraceAccountOption
  model: string
  upstream_model?: string
  attribution: ModelTraceAttribution
  verdict: ModelTraceVerdict
  api_test: {
    requested: number
    attempted: number
    received: number
    accepted: number
    errors: string[]
  }
  responses: ModelTraceResponseReport[]
}

/** SSE 事件（后端 service.ModelTraceEvent） */
export interface ModelTraceEvent {
  type: 'start' | 'progress' | 'challenge_done' | 'result' | 'error' | 'end'
  text?: string
  data?: unknown
}

/**
 * 获取检测选项：支持的模型 + 可用账号列表
 */
export async function getOptions(): Promise<ModelTraceOptionsResponse> {
  const { data } = await apiClient.get<ModelTraceOptionsResponse>('/admin/model-trace/options')
  return data
}

/**
 * 执行检测。检测耗时较长（需向上游发送多条探针），
 * 后端以 SSE 流式推送进度与最终结果。
 * 调用方通过 onEvent 接收事件；返回值仅在流正常结束时为 true。
 */
export async function detect(
  accountId: number,
  model: string,
  onEvent: (event: ModelTraceEvent) => void,
  signal?: AbortSignal
): Promise<boolean> {
  const response = await fetch(buildApiUrl('/admin/model-trace/detect'), {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${localStorage.getItem('auth_token')}`,
      'Content-Type': 'application/json',
      [ADMIN_UI_REQUEST_HEADER]: '1'
    },
    body: JSON.stringify({ account_id: accountId, model }),
    signal
  })

  if (!response.ok || !response.body) {
    return false
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''

    for (const line of lines) {
      if (line.startsWith('data: ')) {
        const jsonStr = line.slice(6).trim()
        if (jsonStr) {
          try {
            onEvent(JSON.parse(jsonStr) as ModelTraceEvent)
          } catch (error) {
            console.error('Failed to parse ModelTrace SSE event:', error)
          }
        }
      }
    }
  }
  return true
}
