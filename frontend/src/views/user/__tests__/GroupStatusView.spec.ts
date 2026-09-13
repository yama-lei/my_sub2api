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

// chart.js 需要 canvas 2d 上下文，jsdom 没有；曲线渲染另行覆盖，这里 stub 掉。
vi.mock('@/features/group-status/ModelTrendChart.vue', () => ({
  default: { name: 'ModelTrendChart', template: '<div class="trend-chart-stub" />' },
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

function seriesFixture() {
  // 3 个桶：健康桶、降级桶（80%）、无流量桶
  const base = Date.parse('2026-09-12T10:00:00Z')
  return [
    { bucket_start: new Date(base).toISOString(), requests: 100, service_errors: 0, success_rate: 1, decode_speed_tps: 55, ttft_ms: 1500, cache_rate: 0.8 },
    { bucket_start: new Date(base + 30 * 60_000).toISOString(), requests: 80, service_errors: 20, success_rate: 0.8, decode_speed_tps: 40, ttft_ms: 2600, cache_rate: 0.7 },
    { bucket_start: new Date(base + 60 * 60_000).toISOString(), requests: 0, service_errors: 0, success_rate: null, decode_speed_tps: null, ttft_ms: null, cache_rate: null },
  ]
}

function reportFixture() {
  return {
    generated_at: '2026-09-12T12:00:00Z',
    window_hours: 24,
    ema_half_life_hours: 6,
    series_bucket_minutes: 30,
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
        models: [
          {
            model: 'gpt-5.6-sol',
            fast: { requests: 6000, decode_speed_tps: 60, ttft_ms: 1200, cache_rate: 0.9 },
            normal: { requests: 640, decode_speed_tps: 48, ttft_ms: 2100, cache_rate: 0.85 },
            overall: { requests: 6640, decode_speed_tps: 58.8, ttft_ms: 1267, cache_rate: 0.895 },
            series: seriesFixture(),
          },
          {
            model: 'gpt-5.6-luna',
            fast: { requests: 0, decode_speed_tps: null, ttft_ms: null, cache_rate: null },
            normal: { requests: 3000, decode_speed_tps: 45, ttft_ms: 3000, cache_rate: 0.8 },
            overall: { requests: 3000, decode_speed_tps: 45, ttft_ms: 3000, cache_rate: 0.8 },
            series: seriesFixture(),
          },
        ],
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
        models: [],
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

  it('renders each model in its own section with uptime bar and tier table', async () => {
    const wrapper = await mountView()
    // 每个模型独立分区，模型名可见
    expect(wrapper.text()).toContain('Per model')
    expect(wrapper.text()).toContain('gpt-5.6-sol')
    expect(wrapper.text()).toContain('gpt-5.6-luna')
    // uptime 条渲染：每个模型的 3 个桶各自成格
    const ticks = wrapper.findAll('.uptime-tick')
    expect(ticks.length).toBe(6)
    // 无流量桶为灰色
    expect(ticks[2].classes().join(' ')).toContain('bg-gray-200')
    // 模型级 fast 行：60 tok/s / 1.2s / 90.0%
    expect(wrapper.text()).toContain('60 tok/s')
    expect(wrapper.text()).toContain('1.2s')
    expect(wrapper.text()).toContain('90.0%')
  })

  it('orders model sections and shows group-level metrics', async () => {
    const wrapper = await mountView()
    // uptime 99.76% → formatMonitorPercent
    expect(wrapper.text()).toContain('99.8%')
    // 分组 Fast 模式：53.3 tok/s / 1.6s / 78.5%
    expect(wrapper.text()).toContain('53.3 tok/s')
    expect(wrapper.text()).toContain('1.6s')
    expect(wrapper.text()).toContain('78.5%')
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
