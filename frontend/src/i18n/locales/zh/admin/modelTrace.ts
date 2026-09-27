export default {
  modelTrace: {
    title: '模型真假检测',
    intro:
      '基于 ModelTrace 指纹归因：向所选模型发送 3 条「长整数第一反应」探针，与内嵌指纹库比对，判断实际响应的模型是否与名称一致（是否掺水 / 套壳）。检测需要多轮上游调用，通常耗时 1-5 分钟。',
    selectAccount: '选择账号（Key）',
    selectAccountPlaceholder: '选择用于调用检测探针的账号',
    selectAccountHint: '仅列出 Anthropic / OpenAI 平台的 OAuth 与 API Key 账号，检测直接使用该账号凭据请求上游。',
    selectModel: '选择要检测的模型',
    selectModelPlaceholder: '选择要检测的模型名称',
    selectModelHint: '目前仅支持指纹库收录的模型（GPT / Claude 家族共 16 个）。',
    run: '开始检测',
    rerun: '重新检测',
    runningHint: '检测进行中，请勿关闭窗口…',
    costHint: '检测会消耗所选账号的上游额度（约 3-6 次调用）',
    abortedOnClose: '检测已取消',
    claimedModel: '声称模型',
    predictedModel: '指纹判定',
    predictionProbability: '判定概率',
    claimedProbability: '声称模型的概率',
    familyProbabilities: '家族概率',
    topModels: '最匹配的模型',
    usedOutputs: '基于 {count} 份有效回答',
    collectionDetails: '采集详情',
    disclaimer: '结果仅供参考，并非判断模型的决定性证据；未收录模型会被归到最相似的现有候选。',
    verdict: {
      match: '判定：一致 —— 该 Key 返回的与声称模型相符',
      family_mismatch: '疑似掺水 / 降级 —— 家族一致，但具体版本不符',
      mismatch: '疑似套壳 / 假冒 —— 实际响应来自另一家族的模型'
    },
    confidence: {
      high: '置信度高',
      medium: '置信度中',
      low: '置信度低'
    },
    progress: {
      started: '开始检测 {model} …',
      done: '检测完成',
      failed: '检测失败',
      aborted: '已取消'
    },
    table: {
      model: '模型',
      probability: '概率',
      similarity: '指纹相似度',
      claimedTag: '声称'
    }
  }
}
