import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import RelayGroupStatusView from '../RelayGroupStatusView.vue'

const api = vi.hoisted(() => ({ groupStatus: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/relay', () => ({ relayApi: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: api.showError }) }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const group = (over: Record<string, unknown>) => ({
  description: '', platform: 'openai', status: 'active', subscription_type: 'standard', rate_multiplier: 1, supplier_rate: 0, mine: 0, ...over
})
const groups = [
  group({ id: 1, name: 'u7: My relay', kind: 'private', platform: 'composite', total: 1, available: 0, mine: 1,
    accounts: [{ mine: true, id: 3, name: 'My Codex', platform: 'openai', type: 'oauth', state: 'error', error_message: 'token revoked' }] }),
  group({ id: 2, name: 'Public pool', kind: 'public_relay', supplier_rate: 0.9, total: 2, available: 1,
    accounts: [{ mine: false, platform: 'openai', type: 'oauth', state: 'available' }, { mine: false, platform: 'openai', type: 'apikey', state: 'rate_limited', until: '2026-10-23T00:00:00Z' }] }),
  group({ id: 4, name: 'Claude', kind: 'standard', platform: 'anthropic', total: 0, available: 0, accounts: [] })
]

let wrapper: VueWrapper | undefined
function mountView() {
  wrapper = mount(RelayGroupStatusView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' }, EmptyState: { props: ['title'], template: '<div>{{ title }}</div>' },
    PlatformIcon: true, Icon: true, RouterLink: { template: '<a><slot /></a>' }
  } } })
  return wrapper
}
const card = (view: VueWrapper, id: number) => view.get(`[data-group="${id}"]`)

beforeEach(() => { vi.clearAllMocks(); api.groupStatus.mockResolvedValue(groups) })
afterEach(() => { wrapper?.unmount(); wrapper = undefined })

describe('Relay group status', () => {
  it('shows every callable group with its health', async () => {
    const view = mountView(); await flushPromises()
    expect(view.findAll('[data-group]')).toHaveLength(3)
    expect(card(view, 1).text()).toContain('relay.groups.defaultName')
    expect(card(view, 1).text()).toContain('relay.status.health.down')
    expect(card(view, 2).text()).toContain('relay.status.health.degraded')
    expect(card(view, 4).text()).toContain('relay.status.health.empty')
  })

  it('lists own account errors and anonymous rows for other accounts', async () => {
    const view = mountView(); await flushPromises()
    await card(view, 1).get('button').trigger('click')
    expect(card(view, 1).text()).toContain('My Codex')
    expect(card(view, 1).text()).toContain('token revoked')
    await card(view, 2).get('button').trigger('click')
    expect(card(view, 2).findAll('li')).toHaveLength(2)
    expect(card(view, 2).text()).toContain('relay.status.otherAccount')
    expect(card(view, 2).text()).toContain('relay.status.states.rate_limited')
  })

  it('filters by group kind and health', async () => {
    const view = mountView(); await flushPromises()
    await view.findAll('button').find(button => button.text() === 'relay.status.filters.private')!.trigger('click')
    expect(view.findAll('[data-group]').map(node => node.attributes('data-group'))).toEqual(['1'])
    await view.findAll('button').find(button => button.text() === 'relay.status.filters.all')!.trigger('click')
    await view.findAll('button').find(button => button.text().startsWith('relay.status.summary.empty'))!.trigger('click')
    expect(view.findAll('[data-group]').map(node => node.attributes('data-group'))).toEqual(['4'])
  })
})
