<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="max-w-3xl">
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('relay.status.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('relay.status.description') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <RouterLink to="/relay" class="btn btn-secondary">{{ t('relay.status.manage') }}</RouterLink>
          <button class="btn btn-secondary" :disabled="loading" :title="t('relay.status.autoRefresh')" @click="load">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3 md:grid-cols-5">
        <div class="card p-4">
          <p class="text-xs text-gray-500">{{ t('relay.status.summary.groups') }}</p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ groups.length }}</p>
        </div>
        <button
          v-for="level in healthLevels"
          :key="level"
          type="button"
          :class="['card p-4 text-left transition-shadow hover:shadow-md', healthFilter === level ? 'ring-2 ring-primary-500' : '']"
          @click="healthFilter = healthFilter === level ? null : level"
        >
          <p class="flex items-center gap-1.5 text-xs text-gray-500">
            <span :class="['h-2 w-2 rounded-full', HEALTH_DOT[level]]" />{{ t(`relay.status.summary.${level}`) }}
          </p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ healthCounts[level] }}</p>
        </button>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <div class="flex flex-wrap gap-1 rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
          <button
            v-for="kind in kindFilters"
            :key="kind"
            type="button"
            :class="[
              'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
              kindFilter === kind ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'
            ]"
            @click="kindFilter = kind"
          >
            {{ t(`relay.status.filters.${kind}`) }}
          </button>
        </div>
        <input v-model="search" type="search" class="input w-full sm:w-64" :placeholder="t('relay.status.search')" />
      </div>

      <EmptyState v-if="loaded && !visibleGroups.length" :title="t('relay.status.empty')" description="" />

      <div class="grid gap-4 xl:grid-cols-2">
        <section v-for="group in visibleGroups" :key="group.id" class="card p-5" :data-group="group.id">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <PlatformIcon :platform="group.platform as GroupPlatform" size="sm" />
                <h2 class="truncate font-semibold text-gray-900 dark:text-white">{{ groupName(group) }}</h2>
                <span :class="['badge', KIND_BADGE[group.kind]]">{{ t(`relay.status.kinds.${group.kind}`) }}</span>
                <span v-if="group.status !== 'active'" class="badge badge-gray">{{ t('relay.status.groupDisabled') }}</span>
              </div>
              <p class="mt-1 text-xs text-gray-500">{{ t(`relay.status.kindHints.${group.kind}`) }}</p>
            </div>
            <div class="shrink-0 text-right text-xs text-gray-500">
              <p class="font-medium text-gray-700 dark:text-gray-300">{{ priceLabel(group) }}</p>
              <p v-if="group.kind === 'public_relay'">{{ t('relay.status.share', { rate: Math.round(group.supplier_rate * 100) }) }}</p>
            </div>
          </div>
          <p v-if="group.description" class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ group.description }}</p>

          <!-- Health: one segment per account state -->
          <div class="mt-4">
            <div class="flex items-center justify-between text-sm">
              <span :class="['flex items-center gap-1.5 font-medium', HEALTH_TEXT[health(group)]]">
                <span :class="['h-2 w-2 rounded-full', HEALTH_DOT[health(group)]]" />{{ t(`relay.status.health.${health(group)}`) }}
              </span>
              <span class="text-gray-500">
                {{ t('relay.status.available', { available: group.available, total: group.total }) }}<template v-if="group.mine"> · {{ t('relay.status.mineCount', { count: group.mine }) }}</template>
              </span>
            </div>
            <div class="mt-2 flex h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
              <div
                v-for="segment in stateSegments(group)"
                :key="segment.state"
                :class="STATE_BAR[segment.state]"
                :style="{ width: `${(segment.count / group.total) * 100}%` }"
                :title="`${t(`relay.status.states.${segment.state}`)}: ${segment.count}`"
              />
            </div>
            <div v-if="stateSegments(group).length" class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500">
              <span v-for="segment in stateSegments(group)" :key="segment.state" class="flex items-center gap-1">
                <span :class="['h-2 w-2 rounded-sm', STATE_BAR[segment.state]]" />{{ t(`relay.status.states.${segment.state}`) }} {{ segment.count }}
              </span>
            </div>
          </div>

          <div class="mt-4 flex flex-wrap items-center justify-between gap-2">
            <button v-if="group.total" type="button" class="btn btn-ghost btn-sm" @click="toggle(group.id)">
              {{ expanded.has(group.id) ? t('relay.status.hideAccounts') : t('relay.status.showAccounts', { count: group.total }) }}
              <Icon :name="expanded.has(group.id) ? 'chevronUp' : 'chevronDown'" size="sm" class="ml-1" />
            </button>
            <span v-else />
            <RouterLink v-if="group.status === 'active'" to="/keys" class="inline-flex items-center text-sm text-primary-600 hover:underline dark:text-primary-400">
              <Icon name="key" size="sm" class="mr-1" />{{ t('relay.status.createKey') }}
            </RouterLink>
          </div>

          <ul v-if="expanded.has(group.id)" class="mt-3 divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-600">
            <li v-for="(account, index) in sortedAccounts(group)" :key="account.id ?? `other-${index}`" class="px-3 py-2.5 text-sm">
              <div class="flex items-center justify-between gap-3">
                <div class="flex min-w-0 items-center gap-2">
                  <PlatformIcon :platform="account.platform as GroupPlatform" size="xs" />
                  <span v-if="account.mine" class="truncate font-medium text-gray-900 dark:text-white">{{ account.name }}</span>
                  <span v-else class="text-gray-500">{{ t('relay.status.otherAccount') }}</span>
                  <span v-if="account.mine" class="badge badge-primary">{{ t('relay.status.myAccount') }}</span>
                  <span class="text-xs text-gray-400">{{ account.type }}</span>
                </div>
                <div class="flex shrink-0 items-center gap-2">
                  <span v-if="account.until" class="text-xs text-gray-500">{{ t('relay.status.until', { time: formatTime(account.until) }) }}</span>
                  <span :class="['badge', STATE_BADGE[account.state]]">{{ t(`relay.status.states.${account.state}`) }}</span>
                </div>
              </div>
              <p v-if="account.error_message || account.reason" class="mt-1 break-words text-xs text-red-600 dark:text-red-400">
                {{ account.error_message || account.reason }}
              </p>
            </li>
          </ul>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { relayApi, type RelayAccountState, type RelayGroupStatus } from '@/api/relay'
import { useAppStore } from '@/stores/app'
import type { GroupPlatform } from '@/types'

type Kind = RelayGroupStatus['kind']
type Health = 'healthy' | 'degraded' | 'down' | 'empty'

const STATE_ORDER: RelayAccountState[] = ['available', 'rate_limited', 'overloaded', 'cooling', 'quota_exceeded', 'expired', 'paused', 'error', 'disabled']
const STATE_BAR: Record<RelayAccountState, string> = {
  available: 'bg-emerald-500', rate_limited: 'bg-amber-400', overloaded: 'bg-amber-500', cooling: 'bg-sky-400',
  quota_exceeded: 'bg-orange-500', expired: 'bg-gray-400', paused: 'bg-gray-300 dark:bg-dark-500', error: 'bg-red-500', disabled: 'bg-gray-400'
}
const STATE_BADGE: Record<RelayAccountState, string> = {
  available: 'badge-success', rate_limited: 'badge-warning', overloaded: 'badge-warning', cooling: 'badge-warning',
  quota_exceeded: 'badge-warning', expired: 'badge-gray', paused: 'badge-gray', error: 'badge-danger', disabled: 'badge-gray'
}
const KIND_BADGE: Record<Kind, string> = { private: 'badge-primary', public_relay: 'badge-warning', standard: 'badge-gray' }
const HEALTH_DOT: Record<Health, string> = { healthy: 'bg-emerald-500', degraded: 'bg-amber-500', down: 'bg-red-500', empty: 'bg-gray-400' }
const HEALTH_TEXT: Record<Health, string> = {
  healthy: 'text-emerald-600 dark:text-emerald-400', degraded: 'text-amber-600 dark:text-amber-400',
  down: 'text-red-600 dark:text-red-400', empty: 'text-gray-500'
}
const healthLevels: Health[] = ['healthy', 'degraded', 'down', 'empty']
const kindFilters: ('all' | Kind)[] = ['all', 'private', 'public_relay', 'standard']

const { t } = useI18n()
const app = useAppStore()
const loading = ref(false)
const loaded = ref(false)
const groups = ref<RelayGroupStatus[]>([])
const kindFilter = ref<'all' | Kind>('all')
const healthFilter = ref<Health | null>(null)
const search = ref('')
const expanded = ref(new Set<number>())

const health = (group: RelayGroupStatus): Health => {
  if (!group.total) return 'empty'
  if (!group.available) return 'down'
  return group.available < group.total ? 'degraded' : 'healthy'
}
const healthCounts = computed(() => {
  const counts: Record<Health, number> = { healthy: 0, degraded: 0, down: 0, empty: 0 }
  for (const group of groups.value) counts[health(group)]++
  return counts
})
const visibleGroups = computed(() => {
  const query = search.value.trim().toLowerCase()
  return groups.value.filter(group =>
    (kindFilter.value === 'all' || group.kind === kindFilter.value) &&
    (!healthFilter.value || health(group) === healthFilter.value) &&
    (!query || groupName(group).toLowerCase().includes(query))
  )
})

function groupName(group: RelayGroupStatus) {
  if (group.kind !== 'private') return group.name
  const name = group.name.replace(/^u\d+: /, '')
  return name === 'My relay' ? t('relay.groups.defaultName') : name
}
function priceLabel(group: RelayGroupStatus) {
  if (group.kind === 'private') return t('relay.status.free')
  if (group.subscription_type === 'subscription') return t('relay.status.subscription')
  return t('relay.status.price', { rate: group.rate_multiplier })
}
function stateSegments(group: RelayGroupStatus) {
  return STATE_ORDER
    .map(state => ({ state, count: group.accounts.filter(account => account.state === state).length }))
    .filter(segment => segment.count > 0)
}
// Own accounts first, then problems before healthy ones.
function sortedAccounts(group: RelayGroupStatus) {
  return [...group.accounts].sort((a, b) =>
    Number(b.mine) - Number(a.mine) || STATE_ORDER.indexOf(b.state) - STATE_ORDER.indexOf(a.state))
}
function toggle(id: number) {
  const next = new Set(expanded.value)
  if (!next.delete(id)) next.add(id)
  expanded.value = next
}
const formatTime = (value: string) => new Date(value).toLocaleString()

async function load() {
  if (loading.value) return
  loading.value = true
  try {
    groups.value = await relayApi.groupStatus()
    loaded.value = true
  } catch { app.showError(t('relay.loadFailed')) }
  finally { loading.value = false }
}

let poll: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  void load()
  poll = setInterval(() => { if (!document.hidden) void load() }, 30000)
})
onUnmounted(() => { if (poll) clearInterval(poll) })
</script>
