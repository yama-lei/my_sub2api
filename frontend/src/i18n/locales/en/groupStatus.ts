/**
 * Group status page (/status) English copy.
 */
export default {
  groupStatus: {
    title: 'Group Status',
    updating: 'Updating…',
    updatedTo: 'Updated to {time} · {window}h window · EMA half-life {halfLife}h',
    retry: 'Retry',
    emptyTitle: 'No groups available',
    emptyDescription: 'Your account has no available groups. Please contact an administrator.',
    uptimeLabel: 'Success rate (uptime)',
    uptimeDetail:
      'Last {window}h: {success} succeeded · {serviceErrors} service errors (business-limited requests excluded: {totalErrors})',
    methodology:
      'Decode speed, TTFT and cache rate are EMA-weighted averages over the last {window}h (half-life {halfLife}h), so newer calls count more; cache rate = cache_read / (input + cache_creation + cache_read). Decode speed only counts the post-TTFT output phase of streaming requests.',
    health: {
      healthy: 'Healthy',
      degraded: 'Degraded',
      down: 'Down',
      idle: 'No traffic',
    },
    modes: {
      overall: 'Overall',
      fast: 'Fast mode',
      normal: 'Normal mode',
    },
    table: {
      mode: 'Mode',
      requests: 'Requests',
      decodeSpeed: 'Decode speed',
      ttft: 'Avg TTFT',
      cacheRate: 'Cache rate',
    },
  },
}
