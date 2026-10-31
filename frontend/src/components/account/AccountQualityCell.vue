<template>
  <div v-if="metrics && hasData" class="flex min-w-[7.5rem] flex-col gap-0.5 text-[11px] leading-4" :title="tooltip">
    <div class="flex items-center gap-1 whitespace-nowrap">
      <span class="text-gray-500 dark:text-dark-400">{{ t('admin.accounts.quality.adulterationShort') }}</span>
      <span class="font-mono" :class="rateClass">{{ formatRate(metrics.adulteration_rate) }}</span>
      <span
        v-if="metrics.latest_probe"
        class="rounded px-1 font-medium"
        :class="verdictClass(metrics.latest_probe.verdict)"
      >
        {{ t(`admin.accounts.quality.verdicts.${metrics.latest_probe.verdict}`, metrics.latest_probe.verdict) }}
      </span>
    </div>
    <div class="whitespace-nowrap font-mono text-gray-700 dark:text-gray-300">
      {{ formatMs(metrics.p50_ttft_ms) }}
      <span class="text-gray-300 dark:text-gray-600">/</span>
      {{ formatTps(metrics.tps) }}
    </div>
  </div>
  <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountQualityMetrics } from '@/api/admin/accounts'

const props = defineProps<{ metrics: AccountQualityMetrics | null }>()
const { t } = useI18n()

const hasData = computed(() => !!props.metrics && (props.metrics.requests > 0 || props.metrics.downgrades > 0 || !!props.metrics.latest_probe))

const formatRate = (value?: number | null) =>
  typeof value === 'number' && Number.isFinite(value) ? `${(value * 100).toFixed(value > 0 && value < 0.1 ? 1 : 0)}%` : '-'
const formatMs = (value?: number | null) => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return value >= 1000 ? `${(value / 1000).toFixed(1)}s` : `${Math.round(value)}ms`
}
const formatTps = (value?: number | null) =>
  typeof value === 'number' && Number.isFinite(value) ? `${value.toFixed(0)} t/s` : '-'

const rateClass = computed(() => {
  const rate = props.metrics?.adulteration_rate
  if (typeof rate !== 'number' || rate <= 0) return 'text-gray-700 dark:text-gray-300'
  return rate >= 0.05 ? 'text-red-600 dark:text-red-400' : 'text-amber-600 dark:text-amber-400'
})

const verdictClass = (verdict: string) => {
  switch (verdict) {
    case 'pass':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
    case 'fail':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    case 'error':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
}

const tooltip = computed(() => {
  const m = props.metrics
  if (!m) return ''
  const probe = m.latest_probe
    ? `${t(`admin.accounts.quality.verdicts.${m.latest_probe.verdict}`, m.latest_probe.verdict)} (${new Date(m.latest_probe.created_at).toLocaleString()})`
    : '-'
  return t('admin.accounts.quality.listTooltip', {
    hours: m.window_hours,
    rate: formatRate(m.adulteration_rate),
    ttft: formatMs(m.p50_ttft_ms),
    tps: typeof m.tps === 'number' ? m.tps.toFixed(1) : '-',
    probe
  })
})
</script>
