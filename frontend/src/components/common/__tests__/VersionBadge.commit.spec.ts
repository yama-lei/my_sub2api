import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import VersionBadge from '../VersionBadge.vue'

const auth = vi.hoisted(() => ({ isAdmin: false }))
vi.mock('@/stores', () => ({
  useAuthStore: () => auth,
  useAppStore: () => ({
    currentVersion: '0.2.15',
    fetchVersion: vi.fn(),
    versionLoading: false,
    hasUpdate: false,
    buildType: 'source'
  })
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() })
}))
vi.mock('@/api/admin/system', () => ({
  performUpdate: vi.fn(), restartService: vi.fn(), getRollbackVersions: vi.fn(), rollback: vi.fn()
}))

afterEach(() => vi.unstubAllGlobals())

describe('VersionBadge build commit', () => {
  for (const isAdmin of [false, true]) {
    it(`shows the build hash for ${isAdmin ? 'admins' : 'users'}`, () => {
      auth.isAdmin = isAdmin
      const commit = '1234567890abcdef1234567890abcdef12345678'
      vi.stubGlobal('__GIT_COMMIT__', commit)
      const wrapper = mount(VersionBadge, { props: { version: '0.2.15' } })
      const badge = wrapper.get('[data-testid="git-commit"]')
      expect(badge.text()).toBe('git 1234567890ab')
      expect(badge.attributes('title')).toBe(`Git commit: ${commit}`)
      wrapper.unmount()
    })
  }

  it('hides the hash when no build commit is available', () => {
    auth.isAdmin = false
    vi.stubGlobal('__GIT_COMMIT__', '')
    const wrapper = mount(VersionBadge, { props: { version: '0.2.15' } })
    expect(wrapper.find('[data-testid="git-commit"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
