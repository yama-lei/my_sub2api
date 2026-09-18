<template>
  <div class="card overflow-hidden">
    <div class="flex flex-wrap items-center gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700/50">
      <div class="w-40">
        <Select
          :model-value="sortBy"
          :options="sortOptions"
          @update:model-value="(value: string | number | boolean | null) => emit('update:sortBy', String(value))"
        />
      </div>
      <div class="w-32">
        <Select
          :model-value="limit"
          :options="limitOptions"
          @update:model-value="(value: string | number | boolean | null) => emit('update:limit', Number(value))"
        />
      </div>
      <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('usage.site.maskedHintShort') }}</span>
    </div>

    <DataTable :columns="columns" :data="items" :loading="loading">
      <template #cell-rank="{ row }">
        <span
          class="inline-flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold"
          :class="rankClass(items.indexOf(row))"
        >
          {{ items.indexOf(row) + 1 }}
        </span>
      </template>

      <template #cell-email="{ row }">
        <div class="flex items-center gap-1.5">
          <span class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700 dark:text-gray-400">
            {{ t('usage.site.maskBadge') }}
          </span>
          <span class="text-sm text-gray-900 dark:text-white">{{ row.email || '-' }}</span>
        </div>
      </template>

      <template #cell-requests="{ row }">
        <span class="text-sm tabular-nums text-gray-700 dark:text-gray-300">{{ row.requests.toLocaleString() }}</span>
      </template>

      <template #cell-input_tokens="{ row }">
        <span class="text-sm tabular-nums text-emerald-600 dark:text-emerald-400">{{ formatCompactNumber(row.input_tokens) }}</span>
      </template>

      <template #cell-output_tokens="{ row }">
        <span class="text-sm tabular-nums text-violet-600 dark:text-violet-400">{{ formatCompactNumber(row.output_tokens) }}</span>
      </template>

      <template #cell-cache_tokens="{ row }">
        <span class="text-sm tabular-nums text-sky-600 dark:text-sky-400">{{ formatCompactNumber(row.cache_tokens) }}</span>
      </template>

      <template #cell-total_tokens="{ row }">
        <span class="text-sm font-medium tabular-nums text-gray-900 dark:text-white">{{ formatCompactNumber(row.total_tokens) }}</span>
      </template>

      <template #cell-actual_cost="{ row }">
        <span class="text-sm font-medium tabular-nums text-green-600 dark:text-green-400">${{ row.actual_cost.toFixed(4) }}</span>
      </template>

      <template #empty>
        <EmptyState :message="t('usage.noRecords')" />
      </template>
    </DataTable>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { formatCompactNumber } from '@/utils/format'
import type { Column } from '@/components/common/types'
import type { SiteUsageRankingItem } from '@/api/siteUsage'

defineProps<{
  items: SiteUsageRankingItem[]
  loading?: boolean
  sortBy?: string
  limit?: number
}>()

const emit = defineEmits<{
  'update:sortBy': [value: string]
  'update:limit': [value: number]
}>()

const { t } = useI18n()

const sortOptions: SelectOption[] = [
  { value: 'total_tokens', label: t('admin.usage.tokenRanking.columns.totalTokens') },
  { value: 'actual_cost', label: t('admin.usage.tokenRanking.columns.cost') },
  { value: 'requests', label: t('admin.usage.tokenRanking.columns.requests') },
  { value: 'input_tokens', label: t('admin.usage.tokenRanking.columns.inputTokens') },
  { value: 'output_tokens', label: t('admin.usage.tokenRanking.columns.outputTokens') },
  { value: 'cache_tokens', label: t('admin.usage.tokenRanking.columns.cacheTokens') },
]

const limitOptions: SelectOption[] = [
  { value: 20, label: 'Top 20' },
  { value: 50, label: 'Top 50' },
  { value: 100, label: 'Top 100' },
  { value: 200, label: 'Top 200' },
]

const columns = computed<Column[]>(() => [
  { key: 'rank', label: '#', sortable: false },
  { key: 'email', label: t('admin.usage.user'), sortable: false },
  { key: 'requests', label: t('admin.usage.tokenRanking.columns.requests'), sortable: false },
  { key: 'input_tokens', label: t('admin.usage.tokenRanking.columns.inputTokens'), sortable: false },
  { key: 'output_tokens', label: t('admin.usage.tokenRanking.columns.outputTokens'), sortable: false },
  { key: 'cache_tokens', label: t('admin.usage.tokenRanking.columns.cacheTokens'), sortable: false },
  { key: 'total_tokens', label: t('admin.usage.tokenRanking.columns.totalTokens'), sortable: false },
  { key: 'actual_cost', label: t('admin.usage.tokenRanking.columns.cost'), sortable: false },
])

const RANK_BADGE_CLASSES = [
  'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400',
  'bg-gray-200 text-gray-600 dark:bg-gray-500/20 dark:text-gray-300',
  'bg-orange-100 text-orange-700 dark:bg-orange-500/20 dark:text-orange-400',
]

const rankClass = (index: number): string =>
  RANK_BADGE_CLASSES[index] ?? 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
</script>
