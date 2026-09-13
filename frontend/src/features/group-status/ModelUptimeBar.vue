<template>
  <div class="uptime-bar" role="img" :aria-label="ariaLabel">
    <div class="flex items-end gap-[2px]">
      <span
        v-for="(point, idx) in series"
        :key="idx"
        class="uptime-tick h-6 min-w-0 flex-1 cursor-help rounded-[3px] transition-transform hover:scale-y-110"
        :class="tickClass(point)"
        :title="tickTitle(point)"
        tabindex="0"
      ></span>
    </div>
    <div class="mt-1 flex items-center justify-between text-[10px] text-gray-400 dark:text-gray-500">
      <span>{{ formatTickLabel(series[0]?.bucket_start) }}</span>
      <span v-if="summaryText" class="font-medium tabular-nums text-gray-500 dark:text-gray-400">{{ summaryText }}</span>
      <span>{{ t('groupStatus.uptimeNow') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import type { GroupModelSeriesPoint } from '@/api/groupStatus'
import { formatMonitorMs, formatMonitorPercent } from '@/features/channel-monitor-v2/monitorFormat'

const props = defineProps<{
  series: GroupModelSeriesPoint[]
  bucketMinutes: number
}>()

const { t, locale } = useI18n()

/** 与健康带一致的分档：无流量灰、≥99% 绿、≥90% 黄、否则红。 */
function tickClass(point: GroupModelSeriesPoint | undefined) {
  if (!point || point.success_rate === null || point.requests + point.service_errors === 0) {
    return 'bg-gray-200 dark:bg-dark-700'
  }
  const rate = point.success_rate
  if (rate >= 0.99) return 'bg-green-500'
  if (rate >= 0.9) return 'bg-amber-500'
  return 'bg-red-500'
}

function formatTickLabel(value: string | undefined) {
  if (!value) return ''
  return new Intl.DateTimeFormat(locale.value || undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function tickTitle(point: GroupModelSeriesPoint | undefined) {
  if (!point) return ''
  const rangeEnd = new Date(new Date(point.bucket_start).getTime() + props.bucketMinutes * 60_000)
  const head = `${formatTickLabel(point.bucket_start)} → ${new Intl.DateTimeFormat(locale.value || undefined, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(rangeEnd)}`
  if (point.requests + point.service_errors === 0) {
    return `${head}\n${t('groupStatus.uptimeNoTraffic')}`
  }
  const lines = [
    head,
    t('groupStatus.uptimeTick', {
      value: point.success_rate === null ? '-' : formatMonitorPercent(point.success_rate),
      success: point.requests,
      errors: point.service_errors,
    }),
  ]
  if (point.ttft_ms !== null) lines.push(t('groupStatus.table.ttft') + `: ${formatMonitorMs(point.ttft_ms)}`)
  if (point.decode_speed_tps !== null) lines.push(t('groupStatus.table.decodeSpeed') + `: ${point.decode_speed_tps.toFixed(1)} tok/s`)
  return lines.join('\n')
}

const summaryText = computed(() => {
  let success = 0
  let errors = 0
  for (const point of props.series) {
    success += point.requests
    errors += point.service_errors
  }
  if (success + errors === 0) return t('groupStatus.health.idle')
  return formatMonitorPercent(success / (success + errors))
})

const ariaLabel = computed(() =>
  t('groupStatus.uptimeBarAria', {
    window: Math.round((props.series.length * props.bucketMinutes) / 60),
    bucket: props.bucketMinutes,
  }),
)
</script>
