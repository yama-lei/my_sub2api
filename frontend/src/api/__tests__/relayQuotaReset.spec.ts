import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('../client', () => ({ apiClient: client }))
import { refreshOpenAIQuota, resetOpenAIQuota } from '../admin/accounts'
import { getClaudeResetCredits, redeemClaudeResetCredit } from '../admin/claudeResetCredits'

beforeEach(() => {
  vi.clearAllMocks()
  client.get.mockResolvedValue({ data: {} })
  client.post.mockResolvedValue({ data: {} })
})

describe('native quota scope', () => {
  it.each(['/admin', '/relay'] as const)('uses %s endpoints and preserves reset confirmation request options', async (scope) => {
    await getClaudeResetCredits(8, scope)
    await redeemClaudeResetCredit(8, 'confirmed-operation', scope)
    await refreshOpenAIQuota(8, scope)
    await resetOpenAIQuota(8, scope)
    expect(client.get).toHaveBeenCalledWith(`${scope}/accounts/8/claude/reset-credits`)
    expect(client.post).toHaveBeenCalledWith(`${scope}/accounts/8/claude/reset-credits/redeem`, undefined, {
      headers: { 'Idempotency-Key': 'confirmed-operation' }
    })
    expect(client.post).toHaveBeenCalledWith(`${scope}/openai/accounts/8/quota/refresh`)
    expect(client.post).toHaveBeenCalledWith(`${scope}/openai/accounts/8/reset-quota`, undefined, { timeout: 90000 })
  })
})
