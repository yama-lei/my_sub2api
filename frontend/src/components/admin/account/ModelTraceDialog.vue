<template>
  <BaseDialog
    :show="show"
    :title="t('admin.modelTrace.title')"
    width="wide"
    @close="handleClose"
  >
    <div class="space-y-4">
      <!-- 介绍 -->
      <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
        {{ t('admin.modelTrace.intro') }}
      </p>

      <!-- 第一步：选择账号（Key） -->
      <div class="space-y-1.5">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.modelTrace.selectAccount') }}
        </label>
        <Select
          v-model="selectedAccountId"
          :options="accountOptions"
          :disabled="running || loadingOptions"
          value-key="value"
          label-key="label"
          :placeholder="loadingOptions ? t('common.loading') + '...' : t('admin.modelTrace.selectAccountPlaceholder')"
        />
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.modelTrace.selectAccountHint') }}
        </p>
      </div>

      <!-- 第二步：选择模型 -->
      <div class="space-y-1.5">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.modelTrace.selectModel') }}
        </label>
        <Select
          v-model="selectedModel"
          :options="modelOptions"
          :disabled="running || loadingOptions"
          value-key="value"
          label-key="label"
          :placeholder="loadingOptions ? t('common.loading') + '...' : t('admin.modelTrace.selectModelPlaceholder')"
        />
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.modelTrace.selectModelHint') }}
        </p>
      </div>

      <!-- 进度日志 -->
      <div v-if="progressLines.length" class="max-h-40 space-y-1 overflow-y-auto rounded-lg bg-gray-50 p-3 font-mono text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
        <div v-for="(line, index) in progressLines" :key="index">{{ line }}</div>
      </div>

      <!-- 错误 -->
      <div
        v-if="errorMessage"
        class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-500/40 dark:bg-red-500/10 dark:text-red-300"
      >
        {{ errorMessage }}
      </div>

      <!-- 结果 -->
      <div v-if="result" class="space-y-4">
        <!-- 结论横幅 -->
        <div
          :class="[
            'rounded-xl border p-4',
            verdictBannerClass
          ]"
        >
          <div class="flex items-center gap-2 text-base font-semibold">
            <Icon :name="verdictIcon" size="md" />
            {{ t(`admin.modelTrace.verdict.${result.verdict.status}`) }}
          </div>
          <div class="mt-2 grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
            <div>
              <span class="opacity-70">{{ t('admin.modelTrace.claimedModel') }}:</span>
              <span class="font-medium">{{ result.model }}</span>
            </div>
            <div>
              <span class="opacity-70">{{ t('admin.modelTrace.predictedModel') }}:</span>
              <span class="font-medium">{{ result.attribution.prediction_name }} ({{ result.attribution.prediction }})</span>
            </div>
            <div>
              <span class="opacity-70">{{ t('admin.modelTrace.predictionProbability') }}:</span>
              <span class="font-medium">{{ formatPercent(result.attribution.probability) }}</span>
              <span class="ml-1 text-xs opacity-70">({{ t(`admin.modelTrace.confidence.${result.verdict.confidence}`) }})</span>
            </div>
            <div v-if="result.verdict.status !== 'match'">
              <span class="opacity-70">{{ t('admin.modelTrace.claimedProbability') }}:</span>
              <span class="font-medium">{{ formatPercent(result.verdict.claimed_probability) }}</span>
            </div>
          </div>
        </div>

        <!-- 家族概率 -->
        <div>
          <h4 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-300">{{ t('admin.modelTrace.familyProbabilities') }}</h4>
          <div class="space-y-2">
            <div v-for="family in result.attribution.family_probabilities" :key="family.family" class="flex items-center gap-3">
              <span class="w-20 shrink-0 text-xs text-gray-500 dark:text-gray-400">{{ family.display_name }}</span>
              <div class="h-2 flex-1 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-600">
                <div
                  class="h-full rounded-full bg-primary-500"
                  :style="{ width: `${Math.max(family.probability * 100, 1)}%` }"
                ></div>
              </div>
              <span class="w-16 shrink-0 text-right text-xs font-medium text-gray-700 dark:text-gray-300">
                {{ formatPercent(family.probability) }}
              </span>
            </div>
          </div>
        </div>

        <!-- Top 模型表格 -->
        <div>
          <h4 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-300">
            {{ t('admin.modelTrace.topModels') }}
            <span class="ml-1 text-xs font-normal text-gray-400">
              {{ t('admin.modelTrace.usedOutputs', { count: result.attribution.used_outputs }) }}
            </span>
          </h4>
          <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <table class="w-full text-left text-sm">
              <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-700 dark:text-gray-400">
                <tr>
                  <th class="px-3 py-2">{{ t('admin.modelTrace.table.model') }}</th>
                  <th class="px-3 py-2">{{ t('admin.modelTrace.table.probability') }}</th>
                  <th class="px-3 py-2">{{ t('admin.modelTrace.table.similarity') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-600 dark:bg-dark-800">
                <tr
                  v-for="(item, index) in topResults"
                  :key="item.model"
                  :class="item.model === result.model ? 'bg-primary-50/60 dark:bg-primary-500/10' : ''"
                >
                  <td class="px-3 py-2">
                    <span class="font-medium text-gray-800 dark:text-gray-200">{{ item.display_name }}</span>
                    <span class="ml-1.5 text-xs text-gray-400">{{ item.model }}</span>
                    <span
                      v-if="item.model === result.model"
                      class="ml-1.5 rounded bg-primary-100 px-1.5 py-0.5 text-[10px] font-medium text-primary-700 dark:bg-primary-500/20 dark:text-primary-300"
                    >
                      {{ t('admin.modelTrace.table.claimedTag') }}
                    </span>
                  </td>
                  <td class="px-3 py-2 font-mono text-xs">
                    {{ formatPercent(item.probability) }}
                    <span v-if="index === 0" class="ml-1 text-[10px] text-gray-400">#1</span>
                  </td>
                  <td class="px-3 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">
                    {{ item.profile_similarity.toFixed(4) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- 采集详情 -->
        <details class="rounded-lg border border-gray-200 px-3 py-2 text-xs text-gray-500 dark:border-dark-600 dark:text-gray-400">
          <summary class="cursor-pointer select-none font-medium">
            {{ t('admin.modelTrace.collectionDetails') }}
          </summary>
          <div class="mt-2 space-y-1.5">
            <div v-for="resp in result.responses" :key="resp.challenge_id" class="flex flex-wrap gap-2">
              <span class="font-mono">{{ resp.challenge_id }}</span>
              <span :class="resp.accepted ? 'text-green-600 dark:text-green-400' : 'text-red-500'">
                {{ resp.parsed_numbers }}/{{ resp.expected_count }}
              </span>
              <span class="truncate opacity-60">"{{ resp.excerpt }}"</span>
            </div>
            <div v-for="(error, index) in result.api_test.errors" :key="'err-' + index" class="text-amber-600 dark:text-amber-400">
              {{ error }}
            </div>
            <p class="pt-1 opacity-60">{{ t('admin.modelTrace.disclaimer') }}</p>
          </div>
        </details>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-between">
        <span v-if="running" class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          <LoadingSpinner size="sm" />
          {{ t('admin.modelTrace.runningHint') }}
        </span>
        <span v-else class="text-xs text-gray-400 dark:text-gray-500">{{ t('admin.modelTrace.costHint') }}</span>
        <div class="flex gap-2">
          <button
            v-if="running"
            class="btn btn-secondary btn-sm"
            @click="abort"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            v-else-if="result"
            class="btn btn-secondary btn-sm"
            :disabled="!canRun"
            @click="runDetection"
          >
            {{ t('admin.modelTrace.rerun') }}
          </button>
          <button
            class="btn btn-primary btn-sm"
            :disabled="!canRun || running"
            @click="runDetection"
          >
            <Icon name="play" size="sm" class="mr-1" />
            {{ result ? t('admin.modelTrace.rerun') : t('admin.modelTrace.run') }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { getOptions, detect } from '@/api/admin/modelTrace'
import type {
  ModelTraceAccountOption,
  ModelTraceEvent,
  ModelTraceModelOption,
  ModelTraceOptionsResponse,
  ModelTraceResult
} from '@/api/admin/modelTrace'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()
const appStore = useAppStore()

const loadingOptions = ref(false)
const options = ref<ModelTraceOptionsResponse | null>(null)
const selectedAccountId = ref<number | null>(null)
const selectedModel = ref<string>('')
const running = ref(false)
const progressLines = ref<string[]>([])
const errorMessage = ref('')
const result = ref<ModelTraceResult | null>(null)
let abortController: AbortController | null = null

const accountOptions = computed(() => {
  const accounts: ModelTraceAccountOption[] = options.value?.accounts ?? []
  return accounts.map((account) => ({
    value: account.id,
    label: `#${account.id} ${account.name}（${account.platform} / ${account.credential_type}）`
  }))
})

const modelOptions = computed(() => {
  const models: ModelTraceModelOption[] = options.value?.models ?? []
  return models.map((model) => ({
    value: model.id,
    label: `${model.display_name}（${model.id}）`
  }))
})

const canRun = computed(() => selectedAccountId.value !== null && selectedModel.value !== '')

const topResults = computed(() => (result.value ? result.value.attribution.results.slice(0, 5) : []))

const verdictBannerClass = computed(() => {
  switch (result.value?.verdict.status) {
    case 'match':
      return 'border-green-200 bg-green-50 text-green-800 dark:border-green-500/40 dark:bg-green-500/10 dark:text-green-300'
    case 'family_mismatch':
      return 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-300'
    default:
      return 'border-red-200 bg-red-50 text-red-800 dark:border-red-500/40 dark:bg-red-500/10 dark:text-red-300'
  }
})

const verdictIcon = computed(() => {
  switch (result.value?.verdict.status) {
    case 'match':
      return 'check'
    case 'family_mismatch':
      return 'exclamationTriangle'
    default:
      return 'x'
  }
})

const formatPercent = (value: number) => `${(value * 100).toFixed(2)}%`

const loadOptions = async () => {
  if (loadingOptions.value || options.value) return
  loadingOptions.value = true
  try {
    options.value = await getOptions()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('common.unknownError')
  } finally {
    loadingOptions.value = false
  }
}

const addProgress = (text: string) => {
  progressLines.value.push(text)
}

const handleEvent = (event: ModelTraceEvent) => {
  switch (event.type) {
    case 'start':
      addProgress(t('admin.modelTrace.progress.started', { model: event.text ?? '' }))
      break
    case 'progress':
    case 'challenge_done':
      addProgress(event.text ?? '')
      break
    case 'result':
      result.value = (event.data as ModelTraceResult) ?? null
      addProgress(t('admin.modelTrace.progress.done'))
      break
    case 'error':
      errorMessage.value = event.text ?? t('common.unknownError')
      addProgress(t('admin.modelTrace.progress.failed'))
      break
    case 'end':
      running.value = false
      break
  }
}

const runDetection = async () => {
  if (!canRun.value || running.value) return
  running.value = true
  result.value = null
  errorMessage.value = ''
  progressLines.value = []
  abortController = new AbortController()
  try {
    await detect(selectedAccountId.value as number, selectedModel.value, handleEvent, abortController.signal)
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      addProgress(t('admin.modelTrace.progress.aborted'))
    } else {
      errorMessage.value = error instanceof Error ? error.message : t('common.unknownError')
    }
  } finally {
    running.value = false
    abortController = null
  }
}

const abort = () => {
  abortController?.abort()
}

const handleClose = () => {
  if (running.value) {
    abortController?.abort()
    appStore.showInfo(t('admin.modelTrace.abortedOnClose'))
  }
  emit('close')
}

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      void loadOptions()
    }
  }
)
</script>
