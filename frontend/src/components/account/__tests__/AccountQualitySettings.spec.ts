import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountQualitySettings from '../AccountQualitySettings.vue'
import type { Account } from '@/types'

const { getModelProbeModels, getQualityMetrics, runModelProbe } = vi.hoisted(() => ({
  getModelProbeModels: vi.fn(),
  getQualityMetrics: vi.fn(),
  runModelProbe: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { accounts: { getModelProbeModels, getQualityMetrics, runModelProbe } }
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const account = (extra: Record<string, unknown> = {}): Account =>
  ({
    id: 24,
    name: 'pool',
    platform: 'openai',
    type: 'apikey',
    extra,
    status: 'active'
  }) as unknown as Account

type Exposed = {
  applyToExtra: (target: Record<string, unknown>) => void
  isDirty: () => boolean
}

const mountSettings = (extra: Record<string, unknown> = {}, poolMode = true) =>
  mount(AccountQualitySettings, {
    props: { account: account(extra), poolMode },
    global: { stubs: { Select: true, Icon: true } }
  })

describe('AccountQualitySettings', () => {
  beforeEach(() => {
    getModelProbeModels.mockReset().mockResolvedValue(['gpt-5.5', 'gpt-6-sol'])
    getQualityMetrics.mockReset().mockResolvedValue(null)
    runModelProbe.mockReset()
  })

  it('leaves an untouched account clean', async () => {
    const wrapper = mountSettings({ pool_mode_note: 'x' })
    await flushPromises()
    const exposed = wrapper.vm as unknown as Exposed
    expect(exposed.isDirty()).toBe(false)
    const target: Record<string, unknown> = { pool_mode_note: 'x' }
    exposed.applyToExtra(target)
    expect(target).toEqual({ pool_mode_note: 'x' })
  })

  it('writes probe settings and clamps numbers', async () => {
    const wrapper = mountSettings()
    await flushPromises()
    await wrapper.get('[data-testid="model-probe-model"]').setValue('gpt-6-sol')
    await wrapper.get('[data-testid="model-probe-interval"]').setValue('5')
    await wrapper.get('[data-testid="model-probe-samples"]').setValue('9')
    await wrapper.get('[data-testid="model-downgrade-reroll-max"]').setValue('4')

    const exposed = wrapper.vm as unknown as Exposed
    expect(exposed.isDirty()).toBe(true)
    const target: Record<string, unknown> = {}
    exposed.applyToExtra(target)
    expect(target).toEqual({
      model_probe_model: 'gpt-6-sol',
      model_probe_interval_minutes: 30,
      model_probe_samples: 5,
      model_downgrade_reroll_max: 4
    })
  })

  it('removes keys reset back to defaults', async () => {
    const wrapper = mountSettings({ model_probe_enabled: true, model_probe_samples: 4, model_downgrade_guard: 'failover' })
    await flushPromises()
    await wrapper.get('[data-testid="model-probe-samples"]').setValue('3')

    const target: Record<string, unknown> = { model_probe_enabled: true, model_probe_samples: 4, model_downgrade_guard: 'failover' }
    ;(wrapper.vm as unknown as Exposed).applyToExtra(target)
    expect(target).toEqual({ model_probe_enabled: true, model_downgrade_guard: 'failover' })
  })

  it('hides the downgrade guard for non API-key OpenAI accounts', async () => {
    const wrapper = mount(AccountQualitySettings, {
      props: { account: { ...account(), type: 'oauth' } as Account, poolMode: false },
      global: { stubs: { Select: true, Icon: true } }
    })
    await flushPromises()
    expect(wrapper.find('[data-testid="model-downgrade-guard-mode"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="model-probe-enabled"]').exists()).toBe(true)
  })

  it('runs a probe with the form models and shows the verdict', async () => {
    runModelProbe.mockResolvedValue({
      id: 1,
      account_id: 24,
      kind: 'probe',
      requested_model: 'gpt-6-sol',
      observed_model: 'gpt-6-sol',
      verdict: 'fail',
      created_at: '2026-10-30T00:00:00Z',
      detail: { expected_model: 'gpt-6-sol', expected_probability: 0.02, top: [{ model: 'gpt-5.6-luna', probability: 0.97 }] }
    })
    const wrapper = mountSettings({ model_probe_model: 'gpt-6-sol' })
    await flushPromises()
    await wrapper.get('[data-testid="model-probe-run"]').trigger('click')
    await flushPromises()

    expect(runModelProbe).toHaveBeenCalledWith(24, 'gpt-6-sol', '')
    const latest = wrapper.get('[data-testid="model-probe-latest"]').text()
    expect(latest).toContain('gpt-5.6-luna 97.0%')
    expect(latest).toContain('admin.accounts.quality.verdicts.fail')
  })
})
