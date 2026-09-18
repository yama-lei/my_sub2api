<template>
  <div class="card overflow-hidden">
    <DataTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      server-side-sort
      default-sort-key="created_at"
      default-sort-order="desc"
      @sort="(key, order) => emit('sort', key, order)"
    >
      <template #cell-user="{ row }">
        <div class="flex items-center gap-1.5">
          <span class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700 dark:text-gray-400">
            {{ t('usage.site.maskBadge') }}
          </span>
          <span class="text-sm text-gray-900 dark:text-white">{{ row.user?.email || '-' }}</span>
        </div>
      </template>

      <template #cell-api_key="{ row }">
        <span class="text-sm text-gray-700 dark:text-gray-300">{{ row.api_key?.name || '-' }}</span>
      </template>

      <template #cell-group="{ row }">
        <span v-if="row.group?.name" class="text-sm text-gray-700 dark:text-gray-300">{{ row.group.name }}</span>
        <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
      </template>

      <template #cell-model="{ row }">
        <span v-if="row.model" class="text-sm font-medium text-gray-900 dark:text-white">{{ row.model }}</span>
        <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
      </template>

      <template #cell-stream="{ row }">
        <span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="requestTypeClass(row.request_type)">
          {{ requestTypeLabel(row.request_type) }}
        </span>
      </template>

      <template #cell-tokens="{ row }">
        <div class="space-y-0.5 text-sm">
          <div class="flex items-center gap-2">
            <span class="inline-flex items-center gap-1">
              <Icon name="arrowDown" size="sm" class="h-3.5 w-3.5 text-emerald-500" />
              <span class="font-medium text-gray-900 dark:text-white">{{ row.input_tokens.toLocaleString() }}</span>
            </span>
            <span class="inline-flex items-center gap-1">
              <Icon name="arrowUp" size="sm" class="h-3.5 w-3.5 text-violet-500" />
              <span class="font-medium text-gray-900 dark:text-white">{{ row.output_tokens.toLocaleString() }}</span>
            </span>
          </div>
          <div v-if="row.cache_read_tokens > 0 || row.cache_creation_tokens > 0" class="flex items-center gap-2 text-xs">
            <span v-if="row.cache_read_tokens > 0" class="text-sky-600 dark:text-sky-400">C {{ formatCompactNumber(row.cache_read_tokens) }}</span>
            <span v-if="row.cache_creation_tokens > 0" class="text-amber-600 dark:text-amber-400">W {{ formatCompactNumber(row.cache_creation_tokens) }}</span>
          </div>
        </div>
      </template>

      <template #cell-cost="{ row }">
        <div class="text-sm">
          <div class="font-medium text-green-600 dark:text-green-400">${{ row.actual_cost.toFixed(6) }}</div>
          <div v-if="row.total_cost !== row.actual_cost" class="text-[11px] text-gray-400 dark:text-gray-500">
            ${{ row.total_cost.toFixed(6) }}
          </div>
        </div>
      </template>

      <template #cell-latency="{ row }">
        <div class="text-xs">
          <div v-if="row.first_token_ms != null" class="text-gray-600 dark:text-gray-300">
            {{ t('usage.latencyFirstToken') }} <span class="font-medium tabular-nums">{{ formatDuration(row.first_token_ms) }}</span>
          </div>
          <div class="text-gray-500 dark:text-gray-400">
            {{ t('usage.latencyDuration') }} <span class="font-medium tabular-nums">{{ formatDuration(row.duration_ms ?? 0) }}</span>
          </div>
        </div>
      </template>

      <template #cell-created_at="{ value }">
        <span class="text-xs tabular-nums text-gray-600 dark:text-gray-400">{{ formatDateTime(value) }}</span>
      </template>

      <template #empty>
        <EmptyState :message="t('usage.noRecords')" />
      </template>
    </DataTable>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatCompactNumber, formatDateTime } from '@/utils/format'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import type { Column } from '@/components/common/types'
import type { SiteUsageLog } from '@/api/siteUsage'

defineProps<{
  rows: SiteUsageLog[]
  loading?: boolean
  sortBy?: string
  sortOrder?: 'asc' | 'desc'
}>()

const emit = defineEmits<{
  sort: [key: string, order: 'asc' | 'desc']
}>()

const { t } = useI18n()

const formatDuration = (ms: number | null | undefined): string => {
  if (ms == null) return '-'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)}s`
  const totalSec = Math.round(ms / 1000)
  if (totalSec < 3600) return `${Math.floor(totalSec / 60)}m ${totalSec % 60}s`
  return `${Math.floor(totalSec / 3600)}h ${Math.floor((totalSec % 3600) / 60)}m`
}

const columns: Column[] = [
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user', label: t('admin.usage.user'), sortable: false },
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'group', label: t('admin.usage.group'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'latency', label: t('usage.latency'), sortable: false },
]

const requestTypeLabel = (requestType: string): string => {
  switch (resolveUsageRequestType({ request_type: requestType })) {
    case 'cyber':
      return t('usage.cyber')
    case 'live':
      return t('usage.live')
    case 'ws_v2':
      return t('usage.ws')
    case 'stream':
      return t('usage.stream')
    case 'sync':
      return t('usage.sync')
    default:
      return t('usage.unknown')
  }
}

const requestTypeClass = (requestType: string): string => {
  switch (resolveUsageRequestType({ request_type: requestType })) {
    case 'ws_v2':
      return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-300'
    case 'stream':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
}
</script>
