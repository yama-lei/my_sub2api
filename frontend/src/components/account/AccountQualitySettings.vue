<template>
  <section class="space-y-4 border-t border-gray-200 pt-4 dark:border-dark-600" data-testid="account-quality-settings">
    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.accounts.quality.title') }}
      </h3>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.quality.hint') }}
      </p>
    </div>

    <div
      v-if="metrics"
      class="grid grid-cols-2 gap-x-3 gap-y-1.5 rounded border border-gray-100 p-3 text-xs dark:border-dark-700 sm:grid-cols-4"
      data-testid="account-quality-metrics"
    >
      <div>
        <div class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.adulterationRate') }}</div>
        <div class="font-medium text-gray-900 dark:text-white">{{ formatRate(metrics.adulteration_rate) }}</div>
      </div>
      <div>
        <div class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.ttft') }}</div>
        <div class="font-medium text-gray-900 dark:text-white">
          {{ formatMs(metrics.p50_ttft_ms) }} / {{ formatMs(metrics.p90_ttft_ms) }}
        </div>
      </div>
      <div>
        <div class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.tps') }}</div>
        <div class="font-medium text-gray-900 dark:text-white">{{ formatNumber(metrics.tps) }}</div>
      </div>
      <div>
        <div class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.requests') }}</div>
        <div class="font-medium text-gray-900 dark:text-white">
          {{ metrics.requests }}
          <span class="text-gray-500 dark:text-gray-400">
            ({{ t('admin.accounts.quality.mismatchesShort', { mismatches: metrics.mismatches, downgrades: metrics.downgrades }) }})
          </span>
        </div>
      </div>
      <p class="col-span-2 text-gray-400 sm:col-span-4">
        {{ t('admin.accounts.quality.windowHint', { hours: metrics.window_hours }) }}
      </p>
    </div>

    <!-- Model downgrade guard: OpenAI API key upstreams only -->
    <div v-if="guardSupported" class="space-y-3">
      <div>
        <label class="input-label">{{ t('admin.accounts.quality.guardMode') }}</label>
        <Select v-model="guardMode" data-testid="model-downgrade-guard-mode" :options="guardModeOptions" />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.quality.guardModeHint') }}
        </p>
      </div>
      <div v-if="effectiveGuardMode === 'reroll'">
        <label class="input-label">{{ t('admin.accounts.quality.rerollMax') }}</label>
        <input
          v-model.number="rerollMax"
          type="number"
          min="1"
          :max="MAX_REROLL"
          step="1"
          class="input"
          data-testid="model-downgrade-reroll-max"
        />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.quality.rerollMaxHint', { default: DEFAULT_REROLL, max: MAX_REROLL }) }}
        </p>
      </div>
    </div>

    <!-- ModelTrace fingerprint probe -->
    <div class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
      <div class="flex items-center justify-between gap-4">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quality.probeEnabled') }}</label>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quality.probeEnabledHint') }}
          </p>
        </div>
        <Toggle
          v-model="probeEnabled"
          data-testid="model-probe-enabled"
          :aria-label="t('admin.accounts.quality.probeEnabled')"
        />
      </div>

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <label class="input-label">{{ t('admin.accounts.quality.probeModel') }}</label>
          <input
            v-model.trim="probeModel"
            type="text"
            class="input"
            placeholder="gpt-5.5"
            data-testid="model-probe-model"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.probeModelHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.quality.probeExpectedModel') }}</label>
          <Select
            v-model="probeExpectedModel"
            data-testid="model-probe-expected-model"
            :options="expectedModelOptions"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.probeExpectedModelHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.quality.probeInterval') }}</label>
          <input
            v-model.number="probeIntervalMinutes"
            type="number"
            :min="MIN_INTERVAL"
            :max="MAX_INTERVAL"
            step="30"
            class="input"
            data-testid="model-probe-interval"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quality.probeIntervalHint', { min: MIN_INTERVAL, default: DEFAULT_INTERVAL }) }}
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.quality.probeSamples') }}</label>
          <input
            v-model.number="probeSamples"
            type="number"
            min="1"
            :max="MAX_SAMPLES"
            step="1"
            class="input"
            data-testid="model-probe-samples"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.quality.probeSamplesHint', { default: DEFAULT_SAMPLES, max: MAX_SAMPLES }) }}
          </p>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="probing || !probeModel"
          data-testid="model-probe-run"
          @click="runProbe"
        >
          <Icon name="refresh" size="xs" class="mr-1.5" :class="{ 'animate-spin': probing }" />
          {{ probing ? t('admin.accounts.quality.probeRunning') : t('admin.accounts.quality.probeRunNow') }}
        </button>
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.probeRunHint') }}</span>
      </div>

      <div
        v-if="latestProbe"
        class="rounded border border-gray-100 p-3 text-xs dark:border-dark-700"
        data-testid="model-probe-latest"
      >
        <div class="mb-2 flex items-center justify-between gap-2">
          <span class="font-medium text-gray-900 dark:text-white">{{ t('admin.accounts.quality.latestProbe') }}</span>
          <span class="rounded px-2 py-0.5 font-medium" :class="verdictClass(latestProbe.verdict)">
            {{ verdictLabel(latestProbe.verdict) }}
          </span>
        </div>
        <div class="grid grid-cols-[minmax(5rem,auto)_minmax(0,1fr)] gap-x-3 gap-y-1">
          <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.probeAt') }}</span>
          <span class="text-gray-900 dark:text-white">{{ formatDate(latestProbe.created_at) }}</span>
          <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.probeModel') }}</span>
          <span class="break-words text-gray-900 dark:text-white">
            {{ latestProbe.requested_model || '-' }}
            <template v-if="latestProbe.observed_model && latestProbe.observed_model !== latestProbe.requested_model">
              ↳ {{ latestProbe.observed_model }}
            </template>
          </span>
          <template v-if="probeDetail.expected_model">
            <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.probeExpectedModel') }}</span>
            <span class="text-gray-900 dark:text-white">
              {{ probeDetail.expected_model }} ({{ formatRate(probeDetail.expected_probability) }})
            </span>
          </template>
          <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.prediction') }}</span>
          <span class="break-words text-gray-900 dark:text-white">{{ topSummary }}</span>
          <template v-if="probeDetail.samples?.length">
            <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.quality.samples') }}</span>
            <span class="text-gray-900 dark:text-white">
              {{ t('admin.accounts.quality.samplesSummary', { used: probeDetail.used_outputs ?? 0, total: probeDetail.samples.length }) }}
            </span>
          </template>
        </div>
        <p v-if="probeErrorText" class="mt-2 break-words text-amber-700 dark:text-amber-300">{{ probeErrorText }}</p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AccountQualityEvent, AccountQualityMetrics } from '@/api/admin/accounts'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Account } from '@/types'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'

// Keep in sync with backend/internal/service/openai_model_downgrade_guard.go and account_quality.go.
const DEFAULT_REROLL = 2
const MAX_REROLL = 5
const DEFAULT_INTERVAL = 360
const MIN_INTERVAL = 30
const MAX_INTERVAL = 7 * 24 * 60
const DEFAULT_SAMPLES = 3
const MAX_SAMPLES = 5

type GuardMode = 'default' | 'off' | 'reroll' | 'failover'

interface ProbeDetail {
  expected_model?: string
  expected_probability?: number
  prediction?: string
  prediction_probability?: number
  used_outputs?: number
  top?: Array<{ model: string; probability: number }>
  samples?: Array<{ error?: string; accepted?: boolean }>
  error?: string
}

const props = defineProps<{ account: Account; poolMode: boolean }>()
const { t } = useI18n()
const appStore = useAppStore()

const extra = (props.account.extra || {}) as Record<string, unknown>
const readNumber = (key: string, fallback: number) => {
  const raw = Number(extra[key])
  return Number.isFinite(raw) && raw > 0 ? Math.floor(raw) : fallback
}
const readGuardMode = (): GuardMode => {
  const raw = String(extra.model_downgrade_guard || '').trim().toLowerCase()
  return raw === 'off' || raw === 'reroll' || raw === 'failover' ? raw : 'default'
}

const guardMode = ref<GuardMode>(readGuardMode())
const rerollMax = ref<number>(readNumber('model_downgrade_reroll_max', DEFAULT_REROLL))
const probeEnabled = ref(extra.model_probe_enabled === true)
const probeModel = ref(String(extra.model_probe_model || ''))
const probeExpectedModel = ref(String(extra.model_probe_expected_model || ''))
const probeIntervalMinutes = ref<number>(readNumber('model_probe_interval_minutes', DEFAULT_INTERVAL))
const probeSamples = ref<number>(readNumber('model_probe_samples', DEFAULT_SAMPLES))

const knownModels = ref<string[]>([])
const metrics = ref<AccountQualityMetrics | null>(null)
const latestProbe = ref<AccountQualityEvent | null>(null)
const probing = ref(false)

const guardSupported = computed(() => props.account.platform === 'openai' && props.account.type === 'apikey')
const effectiveGuardMode = computed(() => {
  if (guardMode.value !== 'default') return guardMode.value
  return props.poolMode ? 'reroll' : 'off'
})
const guardModeOptions = computed(() => [
  {
    value: 'default',
    label: t('admin.accounts.quality.guardModes.default', {
      mode: t(`admin.accounts.quality.guardModes.${props.poolMode ? 'reroll' : 'off'}`)
    })
  },
  { value: 'off', label: t('admin.accounts.quality.guardModes.off') },
  { value: 'reroll', label: t('admin.accounts.quality.guardModes.reroll') },
  { value: 'failover', label: t('admin.accounts.quality.guardModes.failover') }
])
const expectedModelOptions = computed(() => [
  { value: '', label: t('admin.accounts.quality.expectedAuto') },
  ...knownModels.value.map(model => ({ value: model, label: model }))
])

const probeDetail = computed<ProbeDetail>(() => (latestProbe.value?.detail || {}) as ProbeDetail)
const topSummary = computed(() => {
  const top = probeDetail.value.top
  if (!top?.length) return probeDetail.value.prediction || '-'
  return top.map(item => `${item.model} ${formatRate(item.probability)}`).join(', ')
})
const probeErrorText = computed(() => {
  if (probeDetail.value.error) return probeDetail.value.error
  return probeDetail.value.samples?.find(sample => sample.error)?.error || ''
})

const formatRate = (value?: number | null) =>
  typeof value === 'number' && Number.isFinite(value) ? `${(value * 100).toFixed(value < 0.1 ? 2 : 1)}%` : '-'
const formatMs = (value?: number | null) =>
  typeof value === 'number' && Number.isFinite(value) ? `${Math.round(value)}ms` : '-'
const formatNumber = (value?: number | null) =>
  typeof value === 'number' && Number.isFinite(value) ? value.toFixed(1) : '-'
const formatDate = (value?: string) => {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}
const verdictLabel = (verdict: string) => t(`admin.accounts.quality.verdicts.${verdict}`, verdict)
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

const clamp = (value: number, min: number, max: number, fallback: number) => {
  if (!Number.isFinite(value) || value <= 0) return fallback
  return Math.min(max, Math.max(min, Math.floor(value)))
}

/**
 * Merges the quality settings into an account extra map. Only keys that differ
 * from their defaults are stored so untouched accounts keep a clean extra.
 */
const applyToExtra = (target: Record<string, unknown>) => {
  if (guardSupported.value) {
    if (guardMode.value === 'default') delete target.model_downgrade_guard
    else target.model_downgrade_guard = guardMode.value
    const reroll = clamp(rerollMax.value, 1, MAX_REROLL, DEFAULT_REROLL)
    if (reroll === DEFAULT_REROLL) delete target.model_downgrade_reroll_max
    else target.model_downgrade_reroll_max = reroll
  }

  if (probeEnabled.value) target.model_probe_enabled = true
  else delete target.model_probe_enabled
  const setOrDelete = (key: string, value: unknown, keep: boolean) => {
    if (keep) target[key] = value
    else delete target[key]
  }
  setOrDelete('model_probe_model', probeModel.value, !!probeModel.value)
  setOrDelete('model_probe_expected_model', probeExpectedModel.value, !!probeExpectedModel.value)
  const interval = clamp(probeIntervalMinutes.value, MIN_INTERVAL, MAX_INTERVAL, DEFAULT_INTERVAL)
  setOrDelete('model_probe_interval_minutes', interval, interval !== DEFAULT_INTERVAL)
  const samples = clamp(probeSamples.value, 1, MAX_SAMPLES, DEFAULT_SAMPLES)
  setOrDelete('model_probe_samples', samples, samples !== DEFAULT_SAMPLES)
}

const MANAGED_KEYS = [
  'model_downgrade_guard',
  'model_downgrade_reroll_max',
  'model_probe_enabled',
  'model_probe_model',
  'model_probe_expected_model',
  'model_probe_interval_minutes',
  'model_probe_samples'
]

/** Reports whether the form differs from what the account extra currently stores. */
const isDirty = () => {
  const after: Record<string, unknown> = { ...extra }
  applyToExtra(after)
  return MANAGED_KEYS.some(key => JSON.stringify(extra[key]) !== JSON.stringify(after[key]))
}

defineExpose({ applyToExtra, isDirty })

const load = async () => {
  // Best effort: the settings stay editable when either lookup fails.
  const [modelsResult, metricsResult] = await Promise.allSettled([
    Promise.resolve().then(() => adminAPI.accounts.getModelProbeModels()),
    Promise.resolve().then(() => adminAPI.accounts.getQualityMetrics(props.account.id, 24 * 7))
  ])
  if (modelsResult.status === 'fulfilled') knownModels.value = modelsResult.value
  if (metricsResult.status === 'fulfilled' && metricsResult.value) {
    metrics.value = metricsResult.value
    latestProbe.value = metricsResult.value.latest_probe ?? null
  }
}

const runProbe = async () => {
  probing.value = true
  try {
    latestProbe.value = await adminAPI.accounts.runModelProbe(props.account.id, probeModel.value, probeExpectedModel.value)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.quality.probeFailed')))
  } finally {
    probing.value = false
  }
}

onMounted(load)
</script>
