<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="max-w-3xl">
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('relay.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('relay.description') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <RouterLink to="/relay-groups" class="btn btn-secondary">
            <Icon name="server" size="md" class="mr-1" />{{ t('relay.viewStatus') }}
          </RouterLink>
          <button class="btn btn-secondary" :disabled="loading" :title="t('relay.refresh')" @click="load">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <!-- Two ways to use an upstream: the core concept of this page -->
      <div class="grid gap-4 md:grid-cols-2">
        <div class="card p-5">
          <div class="flex items-center gap-2">
            <Icon name="lock" size="md" class="text-primary-600 dark:text-primary-400" />
            <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('relay.modes.private.title') }}</h2>
            <span class="badge badge-success">{{ t('relay.modes.private.tag') }}</span>
          </div>
          <ul class="mt-3 space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
            <li v-for="key in ['p1', 'p2', 'p3']" :key="key" class="flex gap-2">
              <Icon name="check" size="sm" class="mt-0.5 shrink-0 text-emerald-500" />
              <span>{{ t(`relay.modes.private.${key}`) }}</span>
            </li>
          </ul>
        </div>
        <div class="card p-5">
          <div class="flex items-center gap-2">
            <Icon name="users" size="md" class="text-amber-600 dark:text-amber-400" />
            <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('relay.modes.shared.title') }}</h2>
            <span class="badge badge-gray">{{ t('relay.modes.shared.tag') }}</span>
          </div>
          <ul class="mt-3 space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
            <li v-for="key in ['p1', 'p2', 'p3']" :key="key" class="flex gap-2">
              <Icon name="check" size="sm" class="mt-0.5 shrink-0 text-amber-500" />
              <span>{{ t(`relay.modes.shared.${key}`) }}</span>
            </li>
          </ul>
        </div>
      </div>

      <!-- Setup guide: expanded until the first upstream is connected -->
      <section class="card p-5">
        <div class="flex items-center justify-between gap-3">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('relay.guide.title') }}</h2>
          <button class="btn btn-ghost btn-sm" @click="guideOpen = !showGuide">
            {{ showGuide ? t('relay.guide.hide') : t('relay.guide.show') }}
            <Icon :name="showGuide ? 'chevronUp' : 'chevronDown'" size="sm" class="ml-1" />
          </button>
        </div>
        <ol v-if="showGuide" class="mt-4 grid gap-4 lg:grid-cols-3">
          <li class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
            <div class="flex items-center gap-2">
              <span :class="stepClass(hasAccounts)">
                <Icon v-if="hasAccounts" name="check" size="sm" />
                <template v-else>1</template>
              </span>
              <p class="font-medium text-gray-900 dark:text-white">{{ t('relay.guide.step1Title') }}</p>
              <span v-if="hasAccounts" class="badge badge-success ml-auto">{{ t('relay.guide.done') }}</span>
            </div>
            <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('relay.guide.step1Desc') }}</p>
            <button v-if="!hasAccounts" class="btn btn-primary btn-sm mt-3" :disabled="loading" @click="openAccount()">
              <Icon name="plus" size="sm" class="mr-1" />{{ t('relay.guide.step1Action') }}
            </button>
          </li>
          <li class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
            <div class="flex items-center gap-2">
              <span :class="stepClass(false)">2</span>
              <p class="font-medium text-gray-900 dark:text-white">{{ t('relay.guide.step2Title') }}</p>
            </div>
            <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('relay.guide.step2Desc') }}</p>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <code v-for="group in privateGroups" :key="group.id" class="rounded bg-gray-100 px-1.5 py-0.5 text-xs dark:bg-dark-700">{{ group.name }}</code>
            </div>
            <RouterLink to="/keys" class="btn btn-secondary btn-sm mt-3">
              <Icon name="key" size="sm" class="mr-1" />{{ t('relay.guide.step2Action') }}
            </RouterLink>
          </li>
          <li class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
            <div class="flex items-center gap-2">
              <span :class="stepClass(false)">3</span>
              <p class="font-medium text-gray-900 dark:text-white">{{ t('relay.guide.step3Title') }}</p>
            </div>
            <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('relay.guide.step3Desc') }}</p>
            <div class="mt-3 flex items-center gap-2">
              <code class="min-w-0 flex-1 truncate rounded bg-gray-100 px-2 py-1.5 text-xs dark:bg-dark-700" :title="apiBaseUrl">{{ apiBaseUrl }}</code>
              <button class="btn btn-secondary btn-sm" @click="copyToClipboard(apiBaseUrl, t('relay.guide.copied'))">
                <Icon name="copy" size="sm" />
              </button>
            </div>
          </li>
        </ol>
      </section>

      <div>
        <div class="flex gap-1 overflow-x-auto border-b border-gray-200 dark:border-dark-600" role="tablist">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            role="tab"
            :aria-selected="activeTab === tab.key"
            :class="[
              '-mb-px whitespace-nowrap border-b-2 px-4 py-2.5 text-sm font-medium transition-colors',
              activeTab === tab.key
                ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
            ]"
            @click="activeTab = tab.key"
          >
            {{ tab.label }}
            <span v-if="tab.count !== undefined" class="ml-1 text-xs text-gray-400">{{ tab.count }}</span>
          </button>
        </div>

        <!-- Upstream accounts -->
        <div v-if="activeTab === 'accounts'" class="mt-4 space-y-4">
          <section class="card p-5">
            <div class="mb-4 flex justify-end">
              <button class="btn btn-primary" :disabled="loading" @click="openAccount()">
                <Icon name="plus" size="md" class="mr-1" />{{ t('relay.accounts.add') }}
              </button>
            </div>
            <EmptyState
              v-if="!accounts.length && !loading"
              :title="t('relay.accounts.emptyTitle')"
              :description="t('relay.accounts.emptyDesc')"
            />
            <div v-else class="overflow-x-auto">
              <table class="w-full text-left text-sm">
                <thead class="text-xs uppercase text-gray-500 dark:text-gray-400">
                  <tr class="border-b border-gray-200 dark:border-dark-600">
                    <th class="p-3 font-medium">{{ t('relay.accounts.account') }}</th>
                    <th class="p-3 font-medium">{{ t('relay.accounts.usage') }}</th>
                    <th class="p-3 font-medium">{{ t('relay.accounts.status') }}</th>
                    <th class="p-3 font-medium">{{ t('relay.accounts.actions') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="account in accounts" :key="account.id" class="border-b border-gray-100 align-top last:border-0 dark:border-dark-700">
                    <td class="p-3">
                      <div class="flex items-center gap-2">
                        <PlatformIcon :platform="account.platform as GroupPlatform" size="sm" />
                        <span class="font-medium text-gray-900 dark:text-white">{{ account.name }}</span>
                      </div>
                      <p class="mt-0.5 text-xs text-gray-500">{{ account.platform }} · {{ account.type }} · #{{ account.id }}</p>
                    </td>
                    <td class="p-3">
                      <div class="flex flex-wrap gap-1.5">
                        <span v-for="group in accountPrivateGroups(account)" :key="group.id" class="badge badge-primary">
                          <Icon name="lock" size="xs" class="mr-1" />{{ t('relay.accounts.privateIn') }} · {{ displayName(group) }}
                        </span>
                        <span v-for="group in accountPublicGroups(account)" :key="group.id" class="badge badge-warning">
                          <Icon name="users" size="xs" class="mr-1" />{{ t('relay.accounts.sharedIn') }} · {{ group.name }} · {{ t('relay.accounts.share', { rate: percent(group.supplier_rate) }) }}
                        </span>
                      </div>
                      <p v-if="!account.group_ids?.length" class="text-xs text-amber-600">{{ t('relay.accounts.notInGroup') }}</p>
                    </td>
                    <td class="p-3"><AccountStatusIndicator :account="account" /></td>
                    <td class="p-3">
                      <div class="flex flex-wrap gap-1.5">
                        <button class="btn btn-secondary btn-sm" @click="openAccount(account)">{{ t('common.edit') }}</button>
                        <button class="btn btn-secondary btn-sm" @click="openAction(account, 'test')">{{ t('relay.accounts.test') }}</button>
                        <button class="btn btn-secondary btn-sm" @click="openAction(account, 'stats')">{{ t('relay.accounts.stats') }}</button>
                        <template v-if="isOAuth(account)">
                          <button class="btn btn-secondary btn-sm" @click="openAction(account, 'reauth')">{{ t('relay.accounts.reauth') }}</button>
                          <button class="btn btn-secondary btn-sm" :disabled="refreshingID !== null" @click="refreshAccount(account)">{{ t('relay.accounts.refreshToken') }}</button>
                        </template>
                        <button class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" :title="t('common.delete')" @click="pendingDelete = { kind: 'account', id: account.id }">
                          <Icon name="trash" size="sm" />
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <Pager v-model="accountPage" :total="accountTotal" :disabled="loading" @change="load" />
          </section>

          <section class="card p-5">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('relay.accounts.publicTitle') }}</h3>
              <RouterLink to="/relay-groups" class="text-sm text-primary-600 hover:underline dark:text-primary-400">{{ t('relay.viewStatus') }} →</RouterLink>
            </div>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('relay.accounts.publicDesc') }}</p>
            <p v-if="!publicGroups.length" class="mt-3 text-sm text-gray-400">{{ t('relay.accounts.publicEmpty') }}</p>
            <div v-else class="mt-3 flex flex-wrap gap-2">
              <div v-for="group in publicGroups" :key="group.id" class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-600">
                <PlatformIcon :platform="group.platform" size="sm" />
                <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
                <span class="badge badge-warning">{{ t('relay.accounts.share', { rate: percent(group.supplier_rate) }) }}</span>
                <span class="text-xs text-gray-500">{{ t('relay.accounts.price', { rate: group.rate_multiplier }) }}</span>
              </div>
            </div>
          </section>
        </div>

        <!-- Private groups -->
        <div v-else-if="activeTab === 'groups'" class="mt-4">
          <section class="card p-5">
            <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
              <p class="max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('relay.groups.intro') }}</p>
              <button class="btn btn-secondary" :disabled="loading" @click="openGroup()">
                <Icon name="plus" size="md" class="mr-1" />{{ t('relay.groups.add') }}
              </button>
            </div>
            <div class="grid gap-3 md:grid-cols-2">
              <div v-for="group in privateGroups" :key="group.id" class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <div class="flex items-center gap-2">
                      <p class="truncate font-medium text-gray-900 dark:text-white">{{ displayName(group) }}</p>
                      <span v-if="group.status !== 'active'" class="badge badge-gray">{{ t('relay.form.disabled') }}</span>
                    </div>
                    <p class="mt-0.5 text-xs text-gray-500">
                      {{ platformLabel(group.platform) }}
                      <template v-if="groupAccountCount(group.id) !== null"> · {{ groupAccountCount(group.id) ? t('relay.groups.accountCount', { count: groupAccountCount(group.id) }) : t('relay.groups.noAccounts') }}</template>
                    </p>
                  </div>
                  <div class="flex shrink-0 gap-1">
                    <button class="btn btn-ghost btn-sm" :title="t('common.edit')" @click="openGroup(group)"><Icon name="edit" size="sm" /></button>
                    <button class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" :title="t('common.delete')" @click="pendingDelete = { kind: 'group', id: group.id }"><Icon name="trash" size="sm" /></button>
                  </div>
                </div>
                <p v-if="group.description" class="mt-2 text-sm text-gray-500">{{ group.description }}</p>
                <RouterLink to="/keys" class="mt-3 inline-flex items-center text-sm text-primary-600 hover:underline dark:text-primary-400">
                  <Icon name="key" size="sm" class="mr-1" />{{ t('relay.groups.createKey') }}
                </RouterLink>
              </div>
            </div>
          </section>
        </div>

        <!-- Sharing earnings -->
        <div v-else class="mt-4 space-y-4">
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('relay.earnings.intro') }}</p>
          <div class="grid gap-4 sm:grid-cols-3">
            <div class="card p-5"><p class="text-sm text-gray-500">{{ t('relay.earnings.total') }}</p><p class="mt-2 text-2xl font-semibold">{{ money(earnings.total_earned) }}</p></div>
            <div class="card p-5"><p class="text-sm text-gray-500">{{ t('relay.earnings.today') }}</p><p class="mt-2 text-2xl font-semibold">{{ money(earnings.today_earned) }}</p></div>
            <div class="card p-5"><p class="text-sm text-gray-500">{{ t('relay.earnings.pending') }}</p><p class="mt-2 text-2xl font-semibold" :class="earnings.pending_earned > 0 ? 'text-amber-600' : ''">{{ money(earnings.pending_earned) }}</p></div>
          </div>
          <p v-if="earnings.pending_earned > 0" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-400">
            {{ t('relay.earnings.pendingNotice', { amount: money(earnings.pending_earned) }) }}
          </p>
          <section class="card p-5">
            <EmptyState v-if="!earnings.items.length" :title="t('relay.earnings.emptyTitle')" :description="t('relay.earnings.emptyDesc')" />
            <div v-else class="overflow-x-auto">
              <table class="w-full text-left text-sm">
                <thead class="text-xs uppercase text-gray-500 dark:text-gray-400">
                  <tr class="border-b border-gray-200 dark:border-dark-600">
                    <th class="p-3 font-medium">{{ t('relay.earnings.time') }}</th>
                    <th class="p-3 font-medium">{{ t('relay.earnings.account') }}</th>
                    <th class="p-3 font-medium">{{ t('relay.earnings.group') }}</th>
                    <th class="p-3 font-medium">{{ t('relay.earnings.charged') }}</th>
                    <th class="p-3 font-medium">{{ t('relay.earnings.earned') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in earnings.items" :key="row.id" class="border-b border-gray-100 last:border-0 dark:border-dark-700">
                    <td class="p-3 whitespace-nowrap">{{ new Date(row.created_at).toLocaleString() }}</td>
                    <td class="p-3">{{ accountLabel(row.account_id) }} <span v-if="row.self_use" class="badge badge-gray ml-1">{{ t('relay.earnings.selfUse') }}</span></td>
                    <td class="p-3">{{ groupLabel(row.group_id) }}</td>
                    <td class="p-3">{{ money(row.charged_amount) }}</td>
                    <td class="p-3">
                      <span class="font-medium text-emerald-600 dark:text-emerald-400">+{{ money(row.amount) }}</span>
                      <span class="ml-1 text-xs text-gray-500">({{ percent(row.supplier_rate) }}%)</span>
                      <span v-if="!row.credited_at" class="badge badge-warning ml-1">{{ t('relay.earnings.pendingTag') }}</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <Pager v-model="earningPage" :total="earnings.total" :disabled="loading" @change="load" />
          </section>
        </div>
      </div>
    </div>

    <CreateAccountModal :show="showCreate" :proxies="[]" :groups="accountGroups" :initial-group-ids="privateGroups.slice(0, 1).map(group => group.id)" @close="showCreate = false" @created="load" />
    <EditAccountModal :show="showAccount" :account="editingAccount" :proxies="[]" :groups="accountGroups" @close="showAccount = false" @updated="load" />
    <ReAuthAccountModal :show="showReAuth" :account="actionAccount" @close="showReAuth = false" @reauthorized="load" />
    <AccountTestModal :show="showTest" :account="actionAccount" @close="showTest = false" />
    <AccountStatsModal :show="showStats" :account="actionAccount" @close="showStats = false" />

    <BaseDialog :show="showGroup" :title="editingGroup ? t('relay.groups.edit') : t('relay.groups.add')" @close="showGroup = false">
      <form id="relay-group-form" class="space-y-4" @submit.prevent="saveGroup">
        <div><label for="relay-group-name" class="input-label">{{ t('relay.form.name') }}</label><input id="relay-group-name" v-model="groupForm.name" required maxlength="70" class="input" /></div>
        <div v-if="!editingGroup">
          <label for="relay-group-platform" class="input-label">{{ t('relay.form.platform') }}</label>
          <select id="relay-group-platform" v-model="groupForm.platform" class="input">
            <option value="composite">{{ t('relay.groups.composite') }}</option>
            <option v-for="platform in platformOptions" :key="platform.value" :value="platform.value">{{ platform.label }}</option>
          </select>
          <p class="input-hint">{{ t('relay.groups.platformHint') }}</p>
        </div>
        <div v-if="editingGroup">
          <label for="relay-group-status" class="input-label">{{ t('relay.form.status') }}</label>
          <select id="relay-group-status" v-model="groupForm.status" class="input"><option value="active">{{ t('relay.form.active') }}</option><option value="disabled">{{ t('relay.form.disabled') }}</option></select>
          <p class="input-hint">{{ t('relay.groups.statusHint') }}</p>
        </div>
        <div v-if="editingGroup"><label for="relay-group-description" class="input-label">{{ t('relay.form.description') }}</label><textarea id="relay-group-description" v-model="groupForm.description" class="input" maxlength="1000" /></div>
      </form>
      <template #footer><button class="btn btn-secondary" @click="showGroup = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" form="relay-group-form" :disabled="saving">{{ t('common.save') }}</button></template>
    </BaseDialog>
    <ConfirmDialog
      :show="!!pendingDelete"
      :title="pendingDelete?.kind === 'group' ? t('relay.deleteGroupTitle') : t('relay.deleteAccountTitle')"
      :message="pendingDelete?.kind === 'group' ? t('relay.deleteGroupMessage') : t('relay.deleteAccountMessage')"
      danger
      @cancel="pendingDelete = null"
      @confirm="remove"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, provide, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { relayApi, type RelayEarnings } from '@/api/relay'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import type { Account, Group, GroupPlatform } from '@/types'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import CreateAccountModal from '@/components/account/CreateAccountModal.vue'
import EditAccountModal from '@/components/account/EditAccountModal.vue'
import ReAuthAccountModal from '@/components/admin/account/ReAuthAccountModal.vue'
import AccountTestModal from '@/components/admin/account/AccountTestModal.vue'
import AccountStatsModal from '@/components/admin/account/AccountStatsModal.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import { accountWorkspaceKey } from '@/composables/useAccountWorkspace'
import { relayAccountWorkspace } from '@/api/relayAccountWorkspace'

provide(accountWorkspaceKey, relayAccountWorkspace)

const PAGE_SIZE = 20

const { t } = useI18n()
const app = useAppStore()
const { copyToClipboard } = useClipboard()
const loading = ref(false)
const loaded = ref(false)
const saving = ref(false)
const accounts = ref<Account[]>([])
const privateGroups = ref<Group[]>([])
const publicGroups = ref<Group[]>([])
const accountPage = ref(1)
const accountTotal = ref(0)
const earningPage = ref(1)
const earnings = ref<RelayEarnings>({ items: [], total: 0, total_earned: 0, today_earned: 0, pending_earned: 0 })
const editingAccount = ref<Account | null>(null)
const editingGroup = ref<Group | null>(null)
const showAccount = ref(false)
const showCreate = ref(false)
const showReAuth = ref(false)
const showTest = ref(false)
const showStats = ref(false)
const actionAccount = ref<Account | null>(null)
const refreshingID = ref<number | null>(null)
const showGroup = ref(false)
const pendingDelete = ref<{ kind: 'account' | 'group'; id: number } | null>(null)
const groupForm = reactive({ name: '', platform: 'composite', description: '', status: 'active' })
const platformOptions = CONCRETE_PLATFORM_OPTIONS
const apiBaseUrl = ref(window.location.origin)

type Tab = 'accounts' | 'groups' | 'earnings'
const activeTab = ref<Tab>('accounts')
const tabs = computed<{ key: Tab; label: string; count?: number }[]>(() => [
  { key: 'accounts', label: t('relay.tabs.accounts'), count: accountTotal.value },
  { key: 'groups', label: t('relay.tabs.groups'), count: privateGroups.value.length },
  { key: 'earnings', label: t('relay.tabs.earnings') }
])

const hasAccounts = computed(() => accountTotal.value > 0)
// null = follow the default (open until the first upstream is connected)
const guideOpen = ref<boolean | null>(null)
const showGuide = computed(() => guideOpen.value ?? (loaded.value && !hasAccounts.value))

const percent = (rate?: number) => Math.round((rate ?? 0) * 100)
const money = (value: number) => `$${value.toFixed(8).replace(/0+$/, '').replace(/\.$/, '.00')}`
const isOAuth = (account: Account) => account.type === 'oauth' || account.type === 'setup-token'
const stepClass = (done: boolean) => [
  'flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold',
  done ? 'bg-emerald-500 text-white' : 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300'
]
const platformLabel = (platform: string) =>
  platform === 'composite' ? t('relay.groups.composite') : platformOptions.find(option => option.value === platform)?.label ?? platform
const displayName = (group: Group) => {
  const name = group.name.replace(/^u\d+: /, '')
  return name === 'My relay' ? t('relay.groups.defaultName') : name
}
const groupLabel = (id: number) => {
  const priv = privateGroups.value.find(group => group.id === id)
  if (priv) return displayName(priv)
  return publicGroups.value.find(group => group.id === id)?.name ?? `#${id}`
}
const accountLabel = (id: number) => {
  const account = accounts.value.find(item => item.id === id)
  return account ? account.name : `#${id}`
}
const accountPrivateGroups = (account: Account) => privateGroups.value.filter(group => account.group_ids?.includes(group.id))
const accountPublicGroups = (account: Account) => publicGroups.value.filter(group => account.group_ids?.includes(group.id))
// Counts are only exact when every owned account fits on the current page.
const groupAccountCount = (id: number) =>
  accounts.value.length < accountTotal.value ? null : accounts.value.filter(account => account.group_ids?.includes(id)).length

// The shared account forms show group names verbatim, so label each group with how it is used.
const accountGroups = computed(() => [
  ...privateGroups.value.map(group => ({ ...group, name: t('relay.groupOption.private', { name: displayName(group) }) })),
  ...publicGroups.value.map(group => ({ ...group, name: t('relay.groupOption.shared', { name: group.name, rate: percent(group.supplier_rate) }) }))
])

const Pager = defineComponent({
  props: { modelValue: { type: Number, required: true }, total: { type: Number, required: true }, disabled: Boolean },
  emits: ['update:modelValue', 'change'],
  setup(props, { emit }) {
    const go = (page: number) => { emit('update:modelValue', page); emit('change') }
    return () => props.total <= PAGE_SIZE ? null : h('div', { class: 'mt-4 flex items-center justify-end gap-3 text-sm' }, [
      h('button', { class: 'btn btn-secondary btn-sm', disabled: props.disabled || props.modelValue <= 1, onClick: () => go(props.modelValue - 1) }, h(Icon, { name: 'chevronLeft', size: 'sm' })),
      h('span', `${props.modelValue} / ${Math.ceil(props.total / PAGE_SIZE)}`),
      h('button', { class: 'btn btn-secondary btn-sm', disabled: props.disabled || props.modelValue * PAGE_SIZE >= props.total, onClick: () => go(props.modelValue + 1) }, h(Icon, { name: 'chevronRight', size: 'sm' }))
    ])
  }
})

async function load() {
  loading.value = true
  try {
    const [groups, rows, income] = await Promise.all([relayApi.groups(), relayApi.accounts(accountPage.value), relayApi.earnings(earningPage.value)])
    privateGroups.value = groups.private_groups
    publicGroups.value = groups.public_groups
    accounts.value = rows.items
    accountTotal.value = rows.total
    earnings.value = income
    loaded.value = true
  } catch { app.showError(t('relay.loadFailed')) }
  finally { loading.value = false }
}

function openAccount(account?: Account) {
  if (!account) { showCreate.value = true; return }
  editingAccount.value = account
  showAccount.value = true
}

function openAction(account: Account, action: 'test' | 'stats' | 'reauth') {
  actionAccount.value = account
  if (action === 'test') showTest.value = true
  else if (action === 'stats') showStats.value = true
  else showReAuth.value = true
}

async function refreshAccount(account: Account) {
  if (refreshingID.value !== null) return
  refreshingID.value = account.id
  try {
    await relayAccountWorkspace.api.accounts.refreshCredentials(account.id)
    app.showSuccess(t('relay.saved'))
    await load()
  } catch (error) {
    app.showError((error as { message?: string }).message || t('relay.saveFailed'))
  } finally { refreshingID.value = null }
}

function openGroup(group?: Group) {
  editingGroup.value = group || null
  Object.assign(groupForm, { name: group ? displayName(group) : '', platform: group?.platform || 'composite', description: group?.description || '', status: group?.status === 'active' || !group ? 'active' : 'disabled' })
  showGroup.value = true
}

async function saveGroup() {
  saving.value = true
  try {
    if (editingGroup.value) await relayApi.updateGroup(editingGroup.value, groupForm.name.trim(), groupForm.description, groupForm.status)
    else await relayApi.createGroup(groupForm.name.trim(), groupForm.platform)
    showGroup.value = false; app.showSuccess(t('relay.saved')); await load()
  } catch (error) { app.showError((error as { message?: string }).message || t('relay.saveFailed')) }
  finally { saving.value = false }
}

async function remove() {
  const target = pendingDelete.value
  if (!target || saving.value) return
  saving.value = true
  try {
    if (target.kind === 'account') await relayApi.deleteAccount(target.id)
    else await relayApi.deleteGroup(target.id)
    pendingDelete.value = null; app.showSuccess(t('relay.deleted')); await load()
  } catch (error) { app.showError((error as { message?: string }).message || t('relay.deleteFailed')) }
  finally { saving.value = false }
}

let poll: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  void load()
  void app.fetchPublicSettings().then(settings => { if (settings?.api_base_url) apiBaseUrl.value = settings.api_base_url }).catch(() => {})
  poll = setInterval(() => {
    if (!document.hidden && !loading.value && !saving.value) {
      void relayApi.earnings(earningPage.value).then(income => { earnings.value = income }).catch(() => {})
    }
  }, 10000)
})
onUnmounted(() => { if (poll) clearInterval(poll) })
</script>
