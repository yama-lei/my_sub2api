/**
 * 分组状态页（/status）中文文案。
 */
export default {
  groupStatus: {
    title: '分组状态',
    updating: '刷新中…',
    updatedTo: '数据更新至 {time} · 统计窗口 {window} 小时 · EMA 半衰期 {halfLife} 小时',
    retry: '重试',
    emptyTitle: '暂无可用分组',
    emptyDescription: '你的账号当前没有可用的分组，请联系管理员开通。',
    uptimeLabel: '成功率（uptime）',
    uptimeDetail:
      '近 {window} 小时：成功 {success} 次 · 服务错误 {serviceErrors} 次（余额/配额等业务限制 {totalErrors} 次不计入成功率）',
    methodology:
      '解码速度、TTFT 与缓存率为近 {window} 小时按 EMA 时间衰减（半衰期 {halfLife} 小时）加权的平均值，越新的调用权重越高；缓存率口径为 cache_read / (input + cache_creation + cache_read)。解码速度只统计流式请求中 TTFT 之后的输出段。',
    health: {
      healthy: '运行正常',
      degraded: '有所波动',
      down: '服务异常',
      idle: '暂无调用',
    },
    modes: {
      overall: '整体',
      fast: 'Fast 模式',
      normal: '普通模式',
    },
    table: {
      mode: '模式',
      requests: '调用数',
      decodeSpeed: '解码速度',
      ttft: '平均 TTFT',
      cacheRate: '缓存率',
    },
  },
}
