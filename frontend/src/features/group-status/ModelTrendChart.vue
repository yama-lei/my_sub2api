<template>
  <div class="relative h-[150px] w-full">
    <canvas :aria-label="t('groupStatus.chartAria')" role="img" ref="canvasRef"></canvas>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Filler,
} from 'chart.js'
import type { ChartConfiguration } from 'chart.js'
import type { GroupModelSeriesPoint } from '@/api/groupStatus'
import { formatMonitorMs } from '@/features/channel-monitor-v2/monitorFormat'

// 模型级时间曲线：decode 速度（左轴，蓝）+ TTFT（右轴，琥珀）。
// 无流量桶断点显示（spanGaps=false），与 uptime 条的灰色桶对应。
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const props = defineProps<{
  series: GroupModelSeriesPoint[]
  bucketMinutes: number
}>()

const { t, locale } = useI18n()
const canvasRef = ref<HTMLCanvasElement | null>(null)
let chart: ChartJS | null = null

const isDark = computed(() =>
  typeof document !== 'undefined' && document.documentElement.classList.contains('dark'),
)

function labelFor(value: string) {
  return new Intl.DateTimeFormat(locale.value || undefined, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function buildConfig(): ChartConfiguration {
  const gridColor = isDark.value ? 'rgba(148, 163, 184, 0.15)' : 'rgba(100, 116, 139, 0.15)'
  const tickColor = isDark.value ? '#94a3b8' : '#64748b'
  const tooltipBg = isDark.value ? 'rgba(15, 23, 42, 0.95)' : 'rgba(255, 255, 255, 0.98)'
  const tooltipFg = isDark.value ? '#e2e8f0' : '#0f172a'
  const labels = props.series.map((p) => labelFor(p.bucket_start))
  const speeds = props.series.map((p) => p.decode_speed_tps)
  const ttfts = props.series.map((p) => p.ttft_ms)

  return {
    type: 'line',
    data: {
      labels,
      datasets: [
        {
          label: t('groupStatus.chart.speedLegend'),
          data: speeds,
          borderColor: '#0ea5e9',
          backgroundColor: 'rgba(14, 165, 233, 0.08)',
          fill: true,
          yAxisID: 'y',
          borderWidth: 2,
          pointRadius: 0,
          pointHitRadius: 8,
          tension: 0.3,
          spanGaps: false,
        },
        {
          label: t('groupStatus.chart.ttftLegend'),
          data: ttfts,
          borderColor: '#f59e0b',
          backgroundColor: 'rgba(245, 158, 11, 0.06)',
          fill: false,
          yAxisID: 'y1',
          borderWidth: 2,
          pointRadius: 0,
          pointHitRadius: 8,
          tension: 0.3,
          spanGaps: false,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: { display: false },
        tooltip: {
          backgroundColor: tooltipBg,
          titleColor: tooltipFg,
          bodyColor: tooltipFg,
          callbacks: {
            label: (item) => {
              if (item.parsed.y == null) return ''
              const ds = item.dataset.label ?? ''
              return item.datasetIndex === 0
                ? `${ds}: ${Number(item.parsed.y).toFixed(1)} tok/s`
                : `${ds}: ${formatMonitorMs(item.parsed.y)}`
            },
          },
        },
      },
      scales: {
        x: {
          grid: { color: gridColor, drawTicks: false },
          ticks: { color: tickColor, maxRotation: 0, autoSkip: true, maxTicksLimit: 6, font: { size: 10 } },
        },
        y: {
          position: 'left',
          beginAtZero: true,
          grid: { color: gridColor, drawTicks: false },
          ticks: { color: '#0ea5e9', maxTicksLimit: 5, font: { size: 10 } },
          title: { display: false },
        },
        y1: {
          position: 'right',
          beginAtZero: true,
          grid: { drawOnChartArea: false },
          ticks: { color: '#f59e0b', maxTicksLimit: 5, font: { size: 10 } },
        },
      },
    },
  }
}

function render() {
  if (!canvasRef.value) return
  chart?.destroy()
  chart = new ChartJS(canvasRef.value, buildConfig())
}

onMounted(render)
watch(() => [props.series, isDark.value], render)

onBeforeUnmount(() => {
  chart?.destroy()
  chart = null
})
</script>
