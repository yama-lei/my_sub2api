import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import GroupStatusView from '../GroupStatusView.vue'

/**
 * 组件级回归测试：分组状态页按后端返回渲染每个可用分组
 * （整体/Fast/普通 三行的解码速度、TTFT、缓存率 + uptime）。
 */
const { getReport } = vi.hoisted(() => ({
  getReport: vi.fn(),
}))

vi.mock('@/api/groupStatus', () => ({
  groupStatusApi: { getReport },
}))

// 布局组件会牵出 router / 侧边栏 / 更多 store，这里只保留插槽。
vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { name: 'AppLayout', template: '<div><slot /></div>' },
}))

// vitest 里 vue-i18n 是 runtime-only 构建（无消息编译器），按项目惯例替换 t()，
// 文案直接取真实的 en locale，保证断言的是真文案。
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
        const found = lookup(key)
        let value = typeof found === 'string' ? found : key
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

function reportFixture() {
  return {
    generated_at: '2026-09-12T12:00:00Z',
    window_hours: 24,
    ema_half_life_hours: 6,
    groups: [
      {
        group_id: 2,
        name: 'Pro',
        platform: 'openai',
        description: 'pro group',
        has_traffic: true,
        status: 'healthy',
        fast: { requests: 9002, decode_speed_tps: 53.3, ttft_ms: 1585, cache_rate: 0.785 },
        normal: { requests: 11897, decode_speed_tps: 49.5, ttft_ms: 2776, cache_rate: 0.869 },
        overall: { requests: 20899, decode_speed_tps: 51.2, ttft_ms: 2280, cache_rate: 0.833 },
        uptime: { success_requests: 20899, error_requests: 50, service_errors: 50, success_rate: 0.9976 },
      },
      {
        group_id: 3,
        name: 'demo',
        platform: 'openai',
        description: '',
        has_traffic: false,
        status: 'idle',
        fast: { requests: 0, decode_speed_tps: null, ttft_ms: null, cache_rate: null },
        normal: { requests: 0, decode_speed_tps: null, ttft_ms: null, cache_rate: null },
        overall: { requests: 0, decode_speed_tps: null, ttft_ms: null, cache_rate: null },
        uptime: { success_requests: 0, error_requests: 0, service_errors: 0, success_rate: null },
      },
    ],
  }
}

async function mountView() {
  const wrapper = mount(GroupStatusView)
  await flushPromises()
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  getReport.mockResolvedValue(reportFixture())
})

describe('GroupStatusView', () => {
  it('renders one card per available group', async () => {
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('Group Status')
    expect(wrapper.text()).toContain('Pro')
    expect(wrapper.text()).toContain('demo')
    expect(getReport).toHaveBeenCalledTimes(1)
  })

  it('shows uptime percent and per-tier metrics for a traffic group', async () => {
    const wrapper = await mountView()
    // uptime 99.76% → formatMonitorPercent
    expect(wrapper.text()).toContain('99.8%')
    // Fast 模式解码速度 53.3 tok/s、TTFT 1.6s、缓存率 78.5%
    expect(wrapper.text()).toContain('53.3 tok/s')
    expect(wrapper.text()).toContain('1.6s')
    expect(wrapper.text()).toContain('78.5%')
    // 普通模式缓存率 86.9%
    expect(wrapper.text()).toContain('86.9%')
    // 健康徽标
    expect(wrapper.text()).toContain('Healthy')
  })

  it('renders idle placeholders for a group without traffic', async () => {
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('No traffic')
    expect(wrapper.text()).toContain('-')
  })

  it('keeps the error state actionable when the API fails', async () => {
    getReport.mockRejectedValueOnce({ message: 'boom' })
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('Retry')
  })
})
