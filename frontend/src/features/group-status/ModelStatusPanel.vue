<template>
  <section
    class="rounded-2xl border border-gray-100 bg-gray-50/50 p-3.5 dark:border-dark-700 dark:bg-dark-900/40 sm:p-4"
  >
    <!-- 模型头：名称 + 调用量 + 窗口 uptime -->
    <header class="mb-2.5 flex flex-wrap items-center justify-between gap-2">
      <div class="flex min-w-0 items-center gap-2">
        <span class="h-2 w-2 shrink-0 rounded-full" :class="headDotClass" aria-hidden="true"></span>
        <strong class="truncate font-mono text-xs font-bold text-gray-900 dark:text-gray-100" :title="model.model">
          {{ model.model }}
        </strong>
      </div>
      <div class="flex shrink-0 items-center gap-2 text-[11px] text-gray-500 dark:text-gray-400">
        <span class="tabular-nums">{{ formatNumber(model.overall.requests) }}</span>
        <span class="text-gray-300 dark:text-dark-600">|</span>
        <span class="font-semibold tabular-nums" :class="headUptimeClass">
          {{ headUptimeText }}
        </span>
      </div>
    </header>

    <!-- uptime 条（status page 风格） -->
    <ModelUptimeBar :series="model.series" :bucket-minutes="bucketMinutes" />

    <!-- 时间曲线：decode 速度 + TTFT -->
    <div class="mt-3">
      <div class="mb-1 flex flex-wrap items-center gap-3 text-[10px] text-gray-500 dark:text-gray-400">
        <span class="inline-flex items-center gap-1">
          <span class="h-1.5 w-3 rounded-full bg-sky-500"></span>{{ t('groupStatus.chart.speedLegend') }}
        </span>
        <span class="inline-flex items-center gap-1">
          <span class="h-1.5 w-3 rounded-full bg-amber-500"></span>{{ t('groupStatus.chart.ttftLegend') }}
        </span>
      </div>
      <ModelTrendChart :series="model.series" :bucket-minutes="bucketMinutes" />
    </div>

    <!-- 分模式指标（与分组汇总表同列） -->
    <div class="mt-2 overflow-x-auto">
      <table class="w-full min-w-[440px] border-collapse text-left">
        <thead>
          <tr class="text-[10px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
            <th class="pb-1.5 pr-3 font-semibold">{{ t('groupStatus.table.mode') }}</th>
            <th class="pb-1.5 pr-3 text-right font-semibold">{{ t('groupStatus.table.requests') }}</th>
            <th class="pb-1.5 pr-3 text-right font-semibold">{{ t('groupStatus.table.decodeSpeed') }}</th>
            <th class="pb-1.5 pr-3 text-right font-semibold">{{ t('groupStatus.table.ttft') }}</th>
            <th class="pb-1.5 text-right font-semibold">{{ t('groupStatus.table.cacheRate') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in rows"
            :key="row.mode"
            class="border-t border-gray-100 dark:border-dark-700"
          >
            <td class="py-2 pr-3">
              <span class="inline-flex items-center gap-1.5 text-xs font-semibold text-gray-700 dark:text-gray-200">
                <span class="h-1.5 w-1.5 rounded-full" :class="row.dotClass" aria-hidden="true"></span>
                {{ t(row.label) }}
              </span>
            </td>
            <td class="summary-value py-2 pr-3 text-right text-xs font-medium tabular-nums text-gray-600 dark:text-gray-300">
              {{ formatNumber(row.stats.requests) }}
            </td>
            <td class="summary-value py-2 pr-3 text-right text-xs font-medium tabular-nums text-gray-600 dark:text-gray-300">
              {{ formatTps(row.stats.decode_speed_tps) }}
            </td>
            <td class="summary-value py-2 pr-3 text-right text-xs font-medium tabular-nums text-gray-600 dark:text-gray-300">
              {{ formatMonitorMs(row.stats.ttft_ms) }}
            </td>
            <td class="summary-value py-2 text-right text-xs font-medium tabular-nums text-gray-600 dark:text-gray-300">
              {{ formatCacheRate(row.stats.cache_rate) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import ModelUptimeBar from '@/features/group-status/ModelUptimeBar.vue'
import ModelTrendChart from '@/features/group-status/ModelTrendChart.vue'
import type { GroupModelStatus, GroupModelSeriesPoint, GroupStatusTierStats } from '@/api/groupStatus'
import {
  formatMonitorMs,
  formatMonitorPercent,
  formatMonitorThroughput,
} from '@/features/channel-monitor-v2/monitorFormat'

const props = defineProps<{
  model: GroupModelStatus
  bucketMinutes: number
}>()

const { t, locale } = useI18n()

interface ModeRow {
  mode: string
  label: string
  dotClass: string
  stats: GroupStatusTierStats
}

const rows = computed<ModeRow[]>(() => [
  { mode: 'overall', label: 'groupStatus.modes.overall', dotClass: 'bg-gray-400 dark:bg-gray-500', stats: props.model.overall },
  { mode: 'fast', label: 'groupStatus.modes.fast', dotClass: 'bg-amber-500', stats: props.model.fast },
  { mode: 'normal', label: 'groupStatus.modes.normal', dotClass: 'bg-blue-500', stats: props.model.normal },
])

const hasTraffic = computed(() =>
  props.model.series.some((p: GroupModelSeriesPoint) => p.requests + p.service_errors > 0),
)

const headUptime = computed(() => {
  if (!hasTraffic.value || props.model.overall.requests === 0) return null
  // 窗口 uptime 与分组口径一致：成功 / (成功 + 服务错误)，错误数取桶合计。
  let serviceErrors = 0
  for (const p of props.model.series) serviceErrors += p.service_errors
  const total = props.model.overall.requests + serviceErrors
  if (total === 0) return null
  return props.model.overall.requests / total
})

const headUptimeText = computed(() => {
  const rate = headUptime.value
  return rate === null ? t('groupStatus.health.idle') : formatMonitorPercent(rate)
})

const headUptimeClass = computed(() => {
  const rate = headUptime.value
  if (rate === null) return 'text-gray-400'
  if (rate >= 0.99) return 'text-green-600 dark:text-green-400'
  if (rate >= 0.9) return 'text-amber-600 dark:text-amber-400'
  return 'text-red-600 dark:text-red-400'
})

const headDotClass = computed(() => {
  const rate = headUptime.value
  if (rate === null) return 'bg-gray-400'
  if (rate >= 0.99) return 'bg-green-500'
  if (rate >= 0.9) return 'bg-amber-500'
  return 'bg-red-500'
})

function formatNumber(value: number) {
  return Intl.NumberFormat(locale.value || undefined, { maximumFractionDigits: 0 }).format(value || 0)
}

function formatCacheRate(value: number | null | undefined) {
  if (value == null) return '-'
  return formatMonitorPercent(value)
}

function formatTps(value: number | null | undefined) {
  if (value == null) return '-'
  return `${formatMonitorThroughput(value)} tok/s`
}
</script>
