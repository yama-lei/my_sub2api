<template>
  <div class="space-y-6">
    <!-- 脱敏提示条 -->
    <div
      data-testid="site-usage-masked-hint"
      class="flex items-start gap-2 rounded-lg border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-300"
    >
      <Icon name="infoCircle" size="sm" class="mt-0.5 shrink-0" />
      <span>{{ t('usage.site.maskedHint') }}</span>
    </div>

    <UsageStatsCards :stats="siteStats" :show-account-cost="false" :strike-standard-cost="false" />

    <!-- 图表区 -->
    <div class="space-y-4">
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-4">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.timeRange') }}:</span>
            <DateRangePicker
              v-model:start-date="startDate"
              v-model:end-date="endDate"
              @change="onDateRangeChange"
            />
          </div>
          <div class="ml-auto flex items-center gap-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.granularity') }}:</span>
            <div class="w-28">
              <Select v-model="granularity" :options="granularityOptions" @change="loadChartData" />
            </div>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <ModelDistributionChart
          v-model:metric="modelDistributionMetric"
          :model-stats="modelStats"
          :loading="modelStatsLoading"
          :show-source-toggle="false"
          :show-metric-toggle="true"
          :enable-breakdown="false"
          :show-account-cost="false"
          :start-date="startDate"
          :end-date="endDate"
        />
        <GroupDistributionChart
          v-model:metric="groupDistributionMetric"
          :group-stats="groupStats"
          :loading="chartsLoading"
          :show-metric-toggle="true"
          :enable-breakdown="false"
          :show-account-cost="false"
          :start-date="startDate"
          :end-date="endDate"
        />
      </div>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <EndpointDistributionChart
          v-model:metric="endpointDistributionMetric"
          :endpoint-stats="endpointStats"
          :loading="chartsLoading"
          :show-source-toggle="false"
          :show-metric-toggle="true"
          :enable-breakdown="false"
          :title="t('usage.endpointDistribution')"
          :start-date="startDate"
          :end-date="endDate"
        />
        <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
      </div>
    </div>

    <!-- 明细 / 排行 -->
    <div class="card p-6">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div class="flex flex-1 flex-wrap items-end gap-4">
          <div class="w-full sm:w-auto sm:min-w-[220px]">
            <label class="input-label">{{ t('usage.model') }}</label>
            <Select v-model="filters.model" :options="modelOptions" searchable @change="applyFilters" />
          </div>
          <div class="w-full sm:w-auto sm:min-w-[200px]">
            <label class="input-label">{{ t('admin.usage.group') }}</label>
            <Select v-model="filters.group_id" :options="groupOptions" searchable @change="applyFilters" />
          </div>
          <div class="w-full sm:w-auto sm:min-w-[180px]">
            <label class="input-label">{{ t('usage.type') }}</label>
            <Select v-model="filters.request_type" :options="requestTypeOptions" @change="applyFilters" />
          </div>
          <div class="w-full sm:w-auto sm:min-w-[180px]">
            <label class="input-label">{{ t('usage.compactionFilter') }}</label>
            <Select v-model="filters.native_compaction_v2" :options="compactionOptions" @change="applyFilters" />
          </div>
        </div>

        <div class="flex w-full flex-wrap items-center justify-end gap-3 sm:w-auto">
          <button type="button" data-testid="site-usage-refresh" @click="refreshData" :disabled="loading" class="btn btn-secondary">
            {{ t('common.refresh') }}
          </button>
          <button type="button" @click="resetFilters" class="btn btn-secondary">
            {{ t('common.reset') }}
          </button>
        </div>
      </div>

      <div class="mt-4 flex gap-2 border-b border-gray-200 dark:border-dark-700">
        <button class="tab" :class="{ 'tab-active': subTab === 'detail' }" @click="subTab = 'detail'">
          {{ t('usage.site.detail') }}
        </button>
        <button class="tab" :class="{ 'tab-active': subTab === 'ranking' }" @click="switchToRanking">
          {{ t('usage.site.ranking') }}
        </button>
      </div>
    </div>

    <template v-if="subTab === 'detail'">
      <SiteUsageTable
        :rows="siteLogs"
        :loading="loading"
        :sort-by="sortBy"
        :sort-order="sortOrder"
        @sort="handleSort"
      />
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </template>

    <SiteUsageRankingTable
      v-else
      :items="rankingItems"
      :loading="rankingLoading"
      :sort-by="rankingSortBy"
      :limit="rankingLimit"
      @update:sortBy="onRankingSortBy"
      @update:limit="onRankingLimit"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { siteUsageAPI } from '@/api/siteUsage'
import { userGroupsAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import UsageStatsCards from '@/components/admin/usage/UsageStatsCards.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import SiteUsageTable from './SiteUsageTable.vue'
import SiteUsageRankingTable from './SiteUsageRankingTable.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { requestTypeToLegacyStream } from '@/utils/usageRequestType'
import type {
  EndpointStat,
  Group,
  GroupStat,
  ModelStat,
  TrendDataPoint
} from '@/types'
import type { SiteUsageLog, SiteUsageQueryParams, SiteUsageRankingItem, SiteUsageStats } from '@/api/siteUsage'

const { t } = useI18n()
const appStore = useAppStore()

type DistributionMetric = 'tokens' | 'actual_cost'

const siteStats = ref<SiteUsageStats | null>(null)
const siteLogs = ref<SiteUsageLog[]>([])
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const groupStats = ref<GroupStat[]>([])
const endpointStats = ref<EndpointStat[]>([])
const rankingItems = ref<SiteUsageRankingItem[]>([])

const loading = ref(false)
const chartsLoading = ref(false)
const modelStatsLoading = ref(false)
const rankingLoading = ref(false)

const subTab = ref<'detail' | 'ranking'>('detail')
const rankingLimit = ref(50)
const rankingSortBy = ref('total_tokens')

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

const getLast24HoursRangeDates = () => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const getGranularityForRange = (start: string, end: string): 'day' | 'hour' => {
  const startTime = new Date(`${start}T00:00:00`).getTime()
  const endTime = new Date(`${end}T00:00:00`).getTime()
  return Math.ceil((endTime - startTime) / (1000 * 60 * 60 * 24)) <= 1 ? 'hour' : 'day'
}

const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const granularity = ref<'day' | 'hour'>(getGranularityForRange(startDate.value, endDate.value))

const modelDistributionMetric = ref<DistributionMetric>('tokens')
const groupDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionMetric = ref<DistributionMetric>('tokens')

const filters = ref<SiteUsageQueryParams>({
  start_date: startDate.value,
  end_date: endDate.value,
  group_id: null,
  model: null,
  request_type: null,
  native_compaction_v2: null,
  billing_type: null,
  billing_mode: null,
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})
const sortBy = ref('created_at')
const sortOrder = ref<'asc' | 'desc'>('desc')

const groups = ref<Group[]>([])
const modelOptionValues = ref<string[]>([])

const granularityOptions = computed<SelectOption[]>(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') },
])
const requestTypeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'live', label: t('usage.live') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
])
const compactionOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.allCompactionTypes') },
  { value: true, label: t('usage.compactionOnly') },
])
const groupOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allGroups') },
  ...groups.value.map((group) => ({ value: group.id, label: group.name })),
])
const modelOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allModels') },
  ...modelOptionValues.value.map((model) => ({ value: model, label: model })),
])

const normalizedFilters = computed<SiteUsageQueryParams>(() => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : null
  return {
    ...filters.value,
    start_date: startDate.value,
    end_date: endDate.value,
    stream: legacyStream === null ? undefined : legacyStream,
  }
})

let abortController: AbortController | null = null
let statsReqSeq = 0
let chartReqSeq = 0
let modelStatsReqSeq = 0
let rankingReqSeq = 0

const loadLogs = async () => {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  loading.value = true
  try {
    const res = await siteUsageAPI.list(
      {
        ...normalizedFilters.value,
        page: pagination.page,
        page_size: pagination.page_size,
        sort_by: sortBy.value,
        sort_order: sortOrder.value,
      },
      { signal: controller.signal }
    )
    if (!controller.signal.aborted) {
      siteLogs.value = res.items
      pagination.total = res.total
    }
  } catch (error: any) {
    if (error?.name !== 'AbortError' && error?.code !== 'ERR_CANCELED') {
      appStore.showError(t('usage.site.loadFailed'))
    }
  } finally {
    if (abortController === controller) loading.value = false
  }
}

const loadStats = async () => {
  const seq = ++statsReqSeq
  try {
    const stats = await siteUsageAPI.getStats(normalizedFilters.value)
    if (seq !== statsReqSeq) return
    siteStats.value = stats
    endpointStats.value = stats.endpoints || []
  } catch (error) {
    if (seq !== statsReqSeq) return
    console.error('Failed to load site usage stats:', error)
    endpointStats.value = []
  }
}

const loadModelStats = async () => {
  const seq = ++modelStatsReqSeq
  modelStatsLoading.value = true
  try {
    const response = await siteUsageAPI.getModelStats(normalizedFilters.value)
    if (seq !== modelStatsReqSeq) return
    modelStats.value = response.models || []
    const set = new Set(modelOptionValues.value)
    for (const item of modelStats.value) {
      if (item.model) set.add(item.model)
    }
    if (filters.value.model) set.add(filters.value.model)
    modelOptionValues.value = Array.from(set).sort()
  } catch (error) {
    if (seq !== modelStatsReqSeq) return
    console.error('Failed to load site model stats:', error)
    modelStats.value = []
  } finally {
    if (seq === modelStatsReqSeq) modelStatsLoading.value = false
  }
}

const loadChartData = async () => {
  const seq = ++chartReqSeq
  chartsLoading.value = true
  try {
    const snapshot = await siteUsageAPI.getSnapshotV2({
      ...normalizedFilters.value,
      granularity: granularity.value,
    })
    if (seq !== chartReqSeq) return
    trendData.value = snapshot.trend || []
    groupStats.value = snapshot.groups || []
  } catch (error) {
    if (seq !== chartReqSeq) return
    console.error('Failed to load site chart data:', error)
    trendData.value = []
    groupStats.value = []
  } finally {
    if (seq === chartReqSeq) chartsLoading.value = false
  }
}

const loadRanking = async () => {
  const seq = ++rankingReqSeq
  rankingLoading.value = true
  try {
    const items = await siteUsageAPI.getRanking({
      ...normalizedFilters.value,
      limit: rankingLimit.value,
      sort_by: rankingSortBy.value,
    })
    if (seq !== rankingReqSeq) return
    rankingItems.value = items || []
  } catch (error) {
    if (seq !== rankingReqSeq) return
    console.error('Failed to load site ranking:', error)
    appStore.showError(t('usage.site.loadFailed'))
    rankingItems.value = []
  } finally {
    if (seq === rankingReqSeq) rankingLoading.value = false
  }
}

const applyFilters = () => {
  pagination.page = 1
  void loadLogs()
  void loadStats()
  void loadModelStats()
  void loadChartData()
  if (subTab.value === 'ranking') void loadRanking()
}

const refreshData = () => {
  void loadLogs()
  void loadStats()
  void loadModelStats()
  void loadChartData()
  if (subTab.value === 'ranking') void loadRanking()
}

const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = {
    start_date: range.start,
    end_date: range.end,
    group_id: null,
    model: null,
    request_type: null,
    native_compaction_v2: null,
    billing_type: null,
    billing_mode: null,
  }
  granularity.value = getGranularityForRange(range.start, range.end)
  applyFilters()
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  filters.value.start_date = range.startDate
  filters.value.end_date = range.endDate
  granularity.value = getGranularityForRange(range.startDate, range.endDate)
  applyFilters()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  void loadLogs()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadLogs()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortBy.value = key
  sortOrder.value = order
  pagination.page = 1
  void loadLogs()
}

const switchToRanking = () => {
  subTab.value = 'ranking'
  if (rankingItems.value.length === 0) void loadRanking()
}

const onRankingSortBy = (value: string) => {
  rankingSortBy.value = value
  void loadRanking()
}

const onRankingLimit = (value: number) => {
  rankingLimit.value = value
  void loadRanking()
}

const loadFilterOptions = async () => {
  try {
    groups.value = await userGroupsAPI.getAvailable()
  } catch (error) {
    console.error('Failed to load site usage filter options:', error)
  }
}

onMounted(() => {
  void loadFilterOptions()
  refreshData()
})

onUnmounted(() => {
  abortController?.abort()
})

watch(subTab, (tab) => {
  if (tab === 'ranking' && rankingItems.value.length === 0) void loadRanking()
})
</script>
