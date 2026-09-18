<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <!-- Ops 风格外壳：标题工具栏 + 筛选行（与渠道状态页一致的 elevated shell） -->
      <section
        class="card sticky top-0 z-20 !rounded-3xl !border-0 p-0 shadow-sm ring-1 ring-gray-900/5 backdrop-blur-sm dark:!bg-dark-800 dark:ring-dark-700 supports-[backdrop-filter]:bg-white/95 dark:supports-[backdrop-filter]:bg-dark-800/95"
      >
        <header class="page-header mb-0 flex flex-wrap items-start justify-between gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
          <div class="min-w-0">
            <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400">
                <Icon name="chart" size="sm" />
              </span>
              {{ t('groupStatus.title') }}
            </h1>
            <div class="page-description mt-1.5 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span class="relative flex h-2 w-2 shrink-0">
                <span
                  class="relative inline-flex h-2 w-2 rounded-full"
                  :class="loading || refreshing ? 'bg-gray-400' : 'bg-green-500'"
                ></span>
              </span>
              <span v-if="refreshing" class="inline-flex items-center gap-1 text-primary-600 dark:text-primary-300">
                <LoadingSpinner size="sm" />
                {{ t('groupStatus.updating') }}
              </span>
              <span v-else-if="report">
                {{
                  t('groupStatus.updatedTo', {
                    time: formatTime(report.generated_at),
                    window: report.window_hours,
                    halfLife: report.ema_half_life_hours,
                  })
                }}
              </span>
              <span v-else class="text-gray-400">{{ t('common.loading') }}</span>
            </div>
          </div>
          <button
            class="btn btn-secondary btn-icon flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600"
            type="button"
            :title="t('common.refresh')"
            :disabled="loading"
            @click="reload(false)"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </header>

        <!-- 单行筛选工具栏（仿渠道状态页）：平台 · 分组 · 清除 -->
        <div class="monitor-toolbar flex flex-nowrap items-center gap-1.5 overflow-x-auto px-4 py-3 sm:gap-2 sm:px-5">
          <FilterMultiSelect
            v-model="selectedPlatforms"
            compact
            :label="t('groupStatus.filters.platform')"
            :all-label="t('groupStatus.filters.allPlatforms')"
            :options="platformOptions"
          />
          <FilterMultiSelect
            v-model="selectedGroupKeys"
            compact
            :label="t('groupStatus.filters.group')"
            :all-label="t('groupStatus.filters.allGroups')"
            :options="groupOptions"
          />
          <button
            type="button"
            class="btn btn-ghost btn-sm shrink-0 !px-2 !py-1 text-xs"
            :disabled="!hasDimensionFilter"
            :class="!hasDimensionFilter ? 'opacity-40' : ''"
            @click="clearDimensionFilters"
          >
            {{ t('groupStatus.filters.clear') }}
          </button>

          <span
            v-if="report && report.groups.length > 0"
            class="ml-auto hidden shrink-0 text-[11px] tabular-nums text-gray-400 dark:text-dark-400 sm:block"
          >
            {{ t('groupStatus.filters.visibleCount', { visible: visibleGroups.length, total: report.groups.length }) }}
          </span>
        </div>
      </section>

      <!-- 总览 KPI（仿渠道状态页 MetricCell：聚合当前筛选下的分组指标） -->
      <section
        v-if="summary"
        class="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5"
        :aria-label="t('groupStatus.title')"
      >
        <MetricCell
          :label="t('groupStatus.kpi.uptime')"
          :value="kpiUptimeText"
          :detail="t('groupStatus.kpi.requestsDetail', { groups: summary.groups, window: reportWindowHours })"
          :state="kpiUptimeState"
        />
        <MetricCell
          :label="t('groupStatus.kpi.ttft')"
          :value="kpiTtftText"
          :detail="t('groupStatus.kpi.weightedDetail', { window: reportWindowHours })"
          :state="kpiTtftState"
        />
        <MetricCell
          :label="t('groupStatus.kpi.decodeSpeed')"
          :value="kpiTpsText"
          :detail="t('groupStatus.kpi.weightedDetail', { window: reportWindowHours })"
        />
        <MetricCell
          :label="t('groupStatus.kpi.cacheRate')"
          :value="kpiCacheText"
          :detail="t('groupStatus.kpi.cacheDetail')"
          :state="kpiCacheState"
        />
        <MetricCell
          :label="t('groupStatus.kpi.requests')"
          :value="kpiRequestsText"
          :detail="t('groupStatus.kpi.requestsDetail', { groups: summary.groups, window: reportWindowHours })"
        />
      </section>

      <!-- 加载中 -->
      <section
        v-if="loading && !report"
        class="card flex min-h-[240px] items-center justify-center !rounded-3xl !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
      >
        <LoadingSpinner size="md" />
      </section>

      <!-- 加载失败 -->
      <section
        v-else-if="error && !report"
        class="card flex min-h-[240px] flex-col items-center justify-center gap-3 !rounded-3xl !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
      >
        <p class="text-sm text-red-500">{{ error }}</p>
        <button type="button" class="btn btn-secondary btn-sm" @click="reload(false)">
          {{ t('groupStatus.retry') }}
        </button>
      </section>

      <!-- 空状态 -->
      <section
        v-else-if="report && report.groups.length === 0"
        class="card flex min-h-[240px] items-center justify-center !rounded-3xl !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
      >
        <EmptyState
          :title="t('groupStatus.emptyTitle')"
          :description="t('groupStatus.emptyDescription')"
        />
      </section>

      <!-- 筛选后为空 -->
      <section
        v-else-if="report && visibleGroups.length === 0"
        class="card flex min-h-[240px] flex-col items-center justify-center gap-3 !rounded-3xl !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
      >
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('groupStatus.filters.allGroups') }}: 0</p>
        <button type="button" class="btn btn-secondary btn-sm" @click="clearDimensionFilters">
          {{ t('groupStatus.filters.clear') }}
        </button>
      </section>

      <!-- 分组卡片 -->
      <template v-else-if="report">
        <section
          v-for="group in visibleGroups"
          :key="group.group_id"
          class="card flex flex-col gap-4 !rounded-3xl !border-0 !p-5 shadow-sm ring-1 ring-gray-900/5 sm:!p-6 dark:!bg-dark-800 dark:ring-dark-700"
        >
          <!-- 卡片头：状态点 + 名称 + 平台 + 健康徽标；右侧 uptime -->
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="flex min-w-0 items-center gap-2.5">
              <span class="status-dot" :class="statusDotClass(group.status)" aria-hidden="true"></span>
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <strong class="truncate text-base font-bold text-gray-900 dark:text-white">{{ group.name }}</strong>
                  <span class="badge badge-gray shrink-0 uppercase">{{ group.platform }}</span>
                  <span class="badge shrink-0" :class="statusBadgeClass(group.status)">
                    {{ t(`groupStatus.health.${group.status}`) }}
                  </span>
                </div>
                <p v-if="group.description" class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ group.description }}
                </p>
              </div>
            </div>
            <div class="text-right">
              <div class="text-xl font-black tabular-nums text-gray-900 dark:text-white">
                {{ formatUptime(group) }}
              </div>
              <div class="text-[11px] text-gray-500 dark:text-gray-400">
                {{ t('groupStatus.uptimeLabel') }}
              </div>
            </div>
          </div>

          <!-- uptime 细条 -->
          <div
            v-if="group.has_traffic && group.uptime.success_rate !== null"
            class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-900"
            role="progressbar"
            :aria-valuenow="Math.round(group.uptime.success_rate * 100)"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-label="t('groupStatus.uptimeLabel')"
          >
            <div
              class="h-full rounded-full transition-[width] duration-500 ease-out"
              :class="uptimeBarClass(group.status)"
              :style="{ width: `${Math.max(2, Math.round(group.uptime.success_rate * 100))}%` }"
            />
          </div>
          <div v-else class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-900">
            <div class="h-full w-full rounded-full bg-gray-200 dark:bg-dark-700" />
          </div>

          <!-- 指标表：模式 × 调用/解码速度/TTFT/缓存率（风格与渠道状态缓存率展示一致） -->
          <div class="overflow-x-auto">
            <table class="w-full min-w-[480px] border-collapse text-left">
              <thead>
                <tr class="text-[10px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  <th class="pb-2 pr-3 font-semibold">{{ t('groupStatus.table.mode') }}</th>
                  <th class="pb-2 pr-3 text-right font-semibold">{{ t('groupStatus.table.requests') }}</th>
                  <th class="pb-2 pr-3 text-right font-semibold">{{ t('groupStatus.table.decodeSpeed') }}</th>
                  <th class="pb-2 pr-3 text-right font-semibold">{{ t('groupStatus.table.ttft') }}</th>
                  <th class="pb-2 text-right font-semibold">{{ t('groupStatus.table.cacheRate') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in modeRows(group)"
                  :key="row.mode"
                  class="border-t border-gray-100 dark:border-dark-700"
                >
                  <td class="py-2.5 pr-3">
                    <span class="inline-flex items-center gap-1.5 text-xs font-semibold text-gray-700 dark:text-gray-200">
                      <span class="h-1.5 w-1.5 rounded-full" :class="row.dotClass" aria-hidden="true"></span>
                      {{ t(row.label) }}
                    </span>
                  </td>
                  <td class="summary-value py-2.5 pr-3 text-right text-xs font-medium tabular-nums text-gray-600 dark:text-gray-300">
                    {{ formatNumber(row.stats.requests) }}
                  </td>
                  <td
                    class="summary-value py-2.5 pr-3 text-right text-xs font-medium tabular-nums text-gray-600 dark:text-gray-300"
                    :title="exactValue(row.stats.decode_speed_tps)"
                  >
                    {{ formatTps(row.stats.decode_speed_tps) }}
                  </td>
                  <td class="summary-value py-2.5 pr-3 text-right text-xs font-medium tabular-nums text-gray-600 dark:text-gray-300">
                    {{ formatMonitorMs(row.stats.ttft_ms) }}
                  </td>
                  <td class="summary-value py-2.5 text-right text-xs font-medium tabular-nums text-gray-600 dark:text-gray-300">
                    {{ formatCacheRate(row.stats.cache_rate) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- 按模型分开展示：uptime 条 + 时间曲线 + 分模式指标 -->
          <div v-if="group.models && group.models.length" class="space-y-3">
            <h3 class="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              {{ t('groupStatus.perModelTitle') }}
              <span class="badge badge-gray shrink-0">{{ group.models.length }}</span>
            </h3>
            <ModelStatusPanel
              v-for="model in group.models"
              :key="model.model"
              :model="model"
              :bucket-minutes="report.series_bucket_minutes"
            />
          </div>
          <p v-else class="text-[11px] text-gray-400 dark:text-gray-500">
            {{ t('groupStatus.modelsEmpty') }}
          </p>

          <!-- 底部说明：uptime 统计口径 -->
          <p v-if="group.has_traffic" class="text-[11px] text-gray-400 dark:text-gray-500">
            {{
              t('groupStatus.uptimeDetail', {
                window: report.window_hours,
                success: group.uptime.success_requests,
                serviceErrors: group.uptime.service_errors,
                totalErrors: group.uptime.error_requests,
              })
            }}
          </p>
        </section>

        <p class="px-2 text-[11px] leading-relaxed text-gray-400 dark:text-gray-500">
          {{ t('groupStatus.methodology', { window: report.window_hours, halfLife: report.ema_half_life_hours }) }}
        </p>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ModelStatusPanel from '@/features/group-status/ModelStatusPanel.vue'
import MetricCell from '@/features/channel-monitor-v2/MetricCell.vue'
import FilterMultiSelect from '@/features/channel-monitor-v2/FilterMultiSelect.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { groupStatusApi } from '@/api/groupStatus'
import type { GroupHealthStatus, GroupStatusReport, GroupStatusTierStats } from '@/api/groupStatus'
import type { HealthState } from '@/api/channelMonitorV2'
import { formatMonitorMs, formatMonitorPercent, formatMonitorThroughput } from '@/features/channel-monitor-v2/monitorFormat'

interface FilterOption {
  value: string
  label: string
  count?: number
}

const AUTO_REFRESH_MS = 60_000

const { t, locale } = useI18n()
const report = ref<GroupStatusReport | null>(null)
const loading = ref(true)
const refreshing = ref(false)
const error = ref('')
let autoTimer: ReturnType<typeof setInterval> | null = null

// ---- 维度筛选（仿渠道状态页 toolbar；仅影响当前视图，不改数据口径） ----
const selectedPlatforms = ref<string[]>([])
const selectedGroupKeys = ref<string[]>([])

const platformOptions = computed<FilterOption[]>(() => {
  if (!report.value) return []
  return Array.from(new Set(report.value.groups.map((g) => g.platform)))
    .sort()
    .map((platform) => ({ value: platform, label: platform.toUpperCase() }))
})

const groupOptions = computed<FilterOption[]>(() => {
  if (!report.value) return []
  return report.value.groups
    .filter((g) => selectedPlatforms.value.length === 0 || selectedPlatforms.value.includes(g.platform))
    .map((g) => ({ value: String(g.group_id), label: g.name }))
})

const hasDimensionFilter = computed(() => selectedPlatforms.value.length > 0 || selectedGroupKeys.value.length > 0)

function clearDimensionFilters() {
  selectedPlatforms.value = []
  selectedGroupKeys.value = []
}

const visibleGroups = computed(() => {
  if (!report.value) return []
  return report.value.groups.filter((g) => {
    if (selectedPlatforms.value.length > 0 && !selectedPlatforms.value.includes(g.platform)) return false
    if (selectedGroupKeys.value.length > 0 && !selectedGroupKeys.value.includes(String(g.group_id))) return false
    return true
  })
})

// ---- 总览 KPI（按调用量加权聚合当前可见分组） ----
interface KpiSummary {
  groups: number
  requests: number
  uptime: number | null
  ttftMs: number | null
  tps: number | null
  cacheRate: number | null
}

const summary = computed<KpiSummary | null>(() => {
  if (!report.value || visibleGroups.value.length === 0) return null
  let requests = 0
  let success = 0
  let serviceErrors = 0
  let weight = 0
  let ttftWeighted = 0
  let tpsWeighted = 0
  let cacheWeighted = 0
  for (const g of visibleGroups.value) {
    requests += g.overall.requests
    success += g.uptime.success_requests
    serviceErrors += g.uptime.service_errors
    if (g.overall.requests > 0) {
      weight += g.overall.requests
      if (g.overall.ttft_ms != null) ttftWeighted += g.overall.ttft_ms * g.overall.requests
      if (g.overall.decode_speed_tps != null) tpsWeighted += g.overall.decode_speed_tps * g.overall.requests
      if (g.overall.cache_rate != null) cacheWeighted += g.overall.cache_rate * g.overall.requests
    }
  }
  const totalErrors = success + serviceErrors
  return {
    groups: visibleGroups.value.length,
    requests,
    uptime: totalErrors > 0 ? success / totalErrors : null,
    ttftMs: weight > 0 && ttftWeighted > 0 ? ttftWeighted / weight : null,
    tps: weight > 0 && tpsWeighted > 0 ? tpsWeighted / weight : null,
    cacheRate: weight > 0 && cacheWeighted > 0 ? cacheWeighted / weight : null,
  }
})

const reportWindowHours = computed(() => report.value?.window_hours ?? 24)

const kpiUptimeText = computed(() => (summary.value?.uptime != null ? formatMonitorPercent(summary.value.uptime) : '-'))
// 与分组健康带同阈值：≥99% 健康、≥90% 波动，否则异常
const kpiUptimeState = computed<HealthState | undefined>(() => {
  const value = summary.value?.uptime
  if (value == null) return undefined
  if (value >= 0.99) return 'healthy'
  if (value >= 0.9) return 'warning'
  return 'critical'
})

const kpiTtftText = computed(() => (summary.value?.ttftMs != null ? formatMonitorMs(summary.value.ttftMs) : '-'))
// TTFT 健康档：≤3s 健康、≤8s 波动（与渠道状态 TTFT 档位同量级）
const kpiTtftState = computed<HealthState | undefined>(() => {
  const value = summary.value?.ttftMs
  if (value == null) return undefined
  if (value <= 3000) return 'healthy'
  if (value <= 8000) return 'warning'
  return 'critical'
})

const kpiTpsText = computed(() => {
  const value = summary.value?.tps
  if (value == null) return '-'
  return `${formatMonitorThroughput(value)} tok/s`
})

const kpiCacheText = computed(() => (summary.value?.cacheRate != null ? formatMonitorPercent(summary.value.cacheRate) : '-'))
// 缓存率健康档：≥50% 健康、≥20% 波动（与渠道状态缓存率展示同量级）
const kpiCacheState = computed<HealthState | undefined>(() => {
  const value = summary.value?.cacheRate
  if (value == null) return undefined
  if (value >= 0.5) return 'healthy'
  if (value >= 0.2) return 'warning'
  return 'critical'
})

const kpiRequestsText = computed(() => (summary.value ? formatNumber(summary.value.requests) : '-'))

async function reload(initial = true) {
  if (initial) {
    loading.value = true
  } else {
    refreshing.value = true
  }
  try {
    report.value = await groupStatusApi.getReport()
    error.value = ''
  } catch (e) {
    error.value = extractApiErrorMessage(e)
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

onMounted(() => {
  reload(true)
  autoTimer = setInterval(() => {
    if (!loading.value && !document.hidden) void reload(false)
  }, AUTO_REFRESH_MS)
})

onBeforeUnmount(() => {
  if (autoTimer) {
    clearInterval(autoTimer)
    autoTimer = null
  }
})

interface ModeRow {
  mode: string
  label: string
  dotClass: string
  stats: GroupStatusTierStats
}

/** 表格行：总体 / Fast / Normal（overall 放在最上，复用同一组指标列）。 */
function modeRows(group: GroupStatusReport['groups'][number]): ModeRow[] {
  return [
    { mode: 'overall', label: 'groupStatus.modes.overall', dotClass: 'bg-gray-400 dark:bg-gray-500', stats: group.overall },
    { mode: 'fast', label: 'groupStatus.modes.fast', dotClass: 'bg-amber-500', stats: group.fast },
    { mode: 'normal', label: 'groupStatus.modes.normal', dotClass: 'bg-blue-500', stats: group.normal },
  ]
}

function statusDotClass(status: GroupHealthStatus) {
  switch (status) {
    case 'healthy':
      return 'bg-green-500'
    case 'degraded':
      return 'bg-amber-500'
    case 'down':
      return 'bg-red-500'
    default:
      return 'bg-gray-400'
  }
}

function statusBadgeClass(status: GroupHealthStatus) {
  switch (status) {
    case 'healthy':
      return 'badge-success'
    case 'degraded':
      return 'badge-warning'
    case 'down':
      return 'badge-danger'
    default:
      return 'badge-gray'
  }
}

function uptimeBarClass(status: GroupHealthStatus) {
  switch (status) {
    case 'healthy':
      return 'bg-green-500'
    case 'degraded':
      return 'bg-amber-500'
    case 'down':
      return 'bg-red-500'
    default:
      return 'bg-gray-400'
  }
}

function formatUptime(group: GroupStatusReport['groups'][number]) {
  if (!group.has_traffic || group.uptime.success_rate === null) return '-'
  return formatMonitorPercent(group.uptime.success_rate)
}

function formatCacheRate(value: number | null | undefined) {
  if (value == null) return '-'
  return formatMonitorPercent(value)
}

function formatTps(value: number | null | undefined) {
  if (value == null) return '-'
  return `${formatMonitorThroughput(value)} tok/s`
}

function formatNumber(value: number) {
  return Intl.NumberFormat(locale.value || undefined, { maximumFractionDigits: 0 }).format(value || 0)
}

function exactValue(value: number | null | undefined) {
  if (value == null) return '-'
  return Intl.NumberFormat(locale.value || undefined, { maximumFractionDigits: 3 }).format(value)
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat(locale.value || undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}
</script>

<style scoped>
.status-dot {
  display: inline-block;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
  flex-shrink: 0;
}
</style>
