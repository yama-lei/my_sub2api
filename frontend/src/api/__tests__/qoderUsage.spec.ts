import { beforeEach, describe, expect, it, vi } from 'vitest'

const get = vi.hoisted(() => vi.fn())
vi.mock('../client', () => ({ apiClient: { get } }))
import { getAccountCredits } from '../admin/qoderCredits'

beforeEach(() => get.mockReset())

describe('Qoder shared usage API', () => {
  it.each(['/admin', '/relay'] as const)('uses the existing account usage endpoint in %s', async (scope) => {
    const row = { id: 8, state: 'fresh', credits: { subscription: { remaining: 42 } } }
    get.mockResolvedValue({ data: { source: 'active', qoder_credits: row } })
    const signal = new AbortController().signal
    expect(await getAccountCredits(8, true, signal, scope)).toEqual(row)
    expect(get).toHaveBeenCalledWith(`${scope}/accounts/8/usage`, {
      params: { force: true }, signal, timeout: 40000
    })
  })

  it('treats missing Credits as unknown instead of an empty balance', async () => {
    get.mockResolvedValue({ data: { source: 'active' } })
    await expect(getAccountCredits(8)).rejects.toThrow('missing Credits')
  })
})
