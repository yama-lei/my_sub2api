import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import SiteUsageTable from '../SiteUsageTable.vue'
import type { SiteUsageLog } from '@/api/siteUsage'

/**
 * 全站用量明细表（脱敏视图）回归测试：
 * 用户邮箱 / Key 名渲染服务端已脱敏的值，且不渲染 IP 等未提供字段。
 */

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const { default: en } = await import('@/i18n/locales/en')

  const lookup = (path: string): unknown =>
    path.split('.').reduce<unknown>((acc, part) => {
      if (acc && typeof acc === 'object') return (acc as Record<string, unknown>)[part]
      return undefined
    }, en)

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let value = lookup(key)
        if (typeof value !== 'string') value = key
        if (params) {
          for (const [name, replacement] of Object.entries(params)) {
            value = value.replace(new RegExp(`\\{${name}\\}`, 'g'), String(replacement))
          }
        }
        return value
      },
      locale: { value: 'en' },
    }),
  }
})

const row: SiteUsageLog = {
  id: 1,
  created_at: '2026-09-18T12:00:00Z',
  user_id: 42,
  user: { id: 42, email: 'al**om' },
  api_key_id: 7,
  api_key: { id: 7, name: 'my**ey' },
  group_id: 2,
  group: { id: 2, name: 'Pro', platform: 'openai' },
  request_id: 'req-1',
  model: 'gpt-5.6-sol',
  request_type: 'stream',
  stream: true,
  billing_type: 0,
  input_tokens: 100,
  output_tokens: 200,
  cache_creation_tokens: 0,
  cache_read_tokens: 50,
  input_cost: 0.001,
  output_cost: 0.002,
  cache_creation_cost: 0,
  cache_read_cost: 0.0001,
  total_cost: 0.0031,
  actual_cost: 0.0031,
  rate_multiplier: 1,
  duration_ms: 1200,
  first_token_ms: 350,
  image_count: 0,
  video_count: 0,
}

describe('SiteUsageTable', () => {
  it('renders masked user email and masked key name with badges', () => {
    const wrapper = mount(SiteUsageTable, { props: { rows: [row] } })
    const text = wrapper.text()
    expect(text).toContain('al**om')
    expect(text).toContain('my**ey')
    // 脱敏徽标
    expect(text).toContain('Masked')
    expect(wrapper.text()).not.toContain('alice@example.com')
  })

  it('renders model, group, tokens and cost', () => {
    const wrapper = mount(SiteUsageTable, { props: { rows: [row] } })
    const text = wrapper.text()
    expect(text).toContain('gpt-5.6-sol')
    expect(text).toContain('Pro')
    expect(text).toContain('100')
    expect(text).toContain('200')
    expect(text).toContain('$0.003100')
  })

  it('renders placeholder columns when user/api_key are missing', () => {
    const wrapper = mount(SiteUsageTable, {
      props: { rows: [{ ...row, user: null, api_key: null, group: null }] },
    })
    expect(wrapper.text()).toContain('gpt-5.6-sol')
  })
})
