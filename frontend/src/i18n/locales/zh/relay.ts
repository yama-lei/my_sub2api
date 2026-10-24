export default {
  relay: {
    title: '我的中转站',
    description: '把你自己的上游账号（API Key 或 OAuth 登录）接入本站，自己免费使用，或共享给其他用户赚取站内余额。',
    viewStatus: '查看全部分组状态',
    refresh: '刷新',

    modes: {
      private: {
        title: '自己用',
        tag: '免费',
        p1: '只有你自己能用，通过绑定到「私有组」的 API 密钥调用',
        p2: '本站不扣你的余额，也不占用你的付费配额',
        p3: '上游自身的费用、额度、并发和限流照常生效'
      },
      shared: {
        title: '共享赚收益',
        tag: '可选',
        p1: '在账号上勾选公共组后，其他用户调用该组时可能使用你的账号',
        p2: '按调用金额获得分成，收益直接加入你的站内余额（不可提现）',
        p3: '不会暴露你的上游凭证；共享与自用共用同一份上游额度'
      }
    },

    guide: {
      title: '三步开始使用',
      show: '查看上手步骤',
      hide: '收起',
      done: '已完成',
      step1Title: '接入上游账号',
      step1Desc: '添加你的上游 API Key 或通过 OAuth 登录账号，并放入私有组。',
      step1Action: '接入上游',
      step2Title: '创建调用密钥',
      step2Desc: '在「API 密钥」页面新建密钥，分组选择你的私有组：',
      step2Action: '去创建密钥',
      step3Title: '用本站地址调用',
      step3Desc: '客户端填写本站 API 地址和刚创建的密钥，请求就会转发到你自己的上游。',
      step3Fallback: '使用本站的 API 地址（与普通密钥相同）',
      copy: '复制',
      copied: '已复制'
    },

    tabs: {
      accounts: '上游账号',
      groups: '私有组',
      earnings: '共享收益'
    },

    accounts: {
      add: '接入上游',
      emptyTitle: '还没有接入上游账号',
      emptyDesc: '接入后即可通过私有组免费使用，或者共享给其他用户赚取收益。',
      account: '账号',
      usage: '用途',
      status: '状态',
      actions: '操作',
      privateIn: '私用',
      sharedIn: '共享',
      notInGroup: '未加入任何分组，暂时不会被调用',
      test: '测试',
      stats: '统计',
      reauth: '重新授权',
      refreshToken: '刷新令牌',
      publicTitle: '可共享的公共组',
      publicDesc: '编辑账号时勾选下列分组即可开始共享，取消勾选即停止共享。账号平台需与分组平台一致。',
      publicEmpty: '管理员暂未开放可共享的公共组。',
      share: '分成 {rate}%',
      price: '价格倍率 {rate}x'
    },

    groups: {
      intro: '私有组决定你的 API 密钥会调用哪些账号。绑定到私有组的密钥只会使用组内属于你的账号，本站不扣费。一般保留默认私有组即可；需要按用途隔离（例如工作/个人）时再新建。',
      add: '新建私有组',
      edit: '编辑私有组',
      createKey: '为该组创建密钥',
      accountCount: '{count} 个账号',
      noAccounts: '组内还没有账号',
      defaultName: '默认私有组',
      composite: '混合（任意平台）',
      platformHint: '「混合」组可以放入任意平台的账号；指定平台的组只接受该平台的账号。创建后不可更改。',
      statusHint: '停用后，绑定该组的密钥将无法调用。',
      deleteHint: '删除后，仍绑定该组的密钥会被拒绝请求。'
    },

    earnings: {
      total: '累计收益',
      today: '今日收益（UTC）',
      pending: '待结算',
      intro: '只有当其他用户（或你自己）通过公共组调用到你共享的账号时才会产生收益。通过私有组自用不产生收益，也不扣费。',
      pendingNotice: '有 {amount} 收益待结算：对方余额不足时先记账，对方补足后自动入账。',
      emptyTitle: '暂无收益',
      emptyDesc: '在账号上勾选公共组开始共享后，被调用时收益会显示在这里。',
      time: '时间',
      account: '账号',
      group: '公共组',
      charged: '调用方支付',
      earned: '你的收益',
      selfUse: '自己调用',
      pendingTag: '待结算'
    },

    form: {
      name: '名称',
      platform: '平台',
      status: '状态',
      description: '说明（可选）',
      active: '启用',
      disabled: '停用'
    },

    status: {
      title: '分组状态',
      description: '这里列出你能调用的全部分组，以及每个分组里账号的实时状态。API 密钥绑定某个分组后，请求只会分配给该组中「可用」的账号；一个可用账号都没有时，请求会失败。',
      refresh: '刷新',
      autoRefresh: '每 30 秒自动刷新',
      search: '搜索分组名称',
      onlyProblems: '只看异常分组',
      filters: { all: '全部', private: '我的私有组', public_relay: '公共共享组', standard: '平台分组' },
      kinds: { private: '私有组', public_relay: '公共共享组', standard: '平台分组' },
      kindHints: {
        private: '仅你可用，只调用你自己的账号，本站不扣费',
        public_relay: '按价格扣费，其他用户也可以共享账号进来',
        standard: '由管理员提供账号，按价格扣费'
      },
      summary: { groups: '可调用分组', healthy: '正常', degraded: '部分异常', down: '无可用账号', empty: '没有账号' },
      health: {
        healthy: '全部可用',
        degraded: '部分账号不可用',
        down: '当前没有可用账号，请求会失败',
        empty: '分组内还没有账号，请求会失败'
      },
      available: '可用 {available} / 共 {total}',
      mineCount: '其中 {count} 个是你的',
      free: '免费',
      price: '{rate}x 价格',
      subscription: '订阅',
      share: '共享分成 {rate}%',
      groupDisabled: '分组已停用',
      showAccounts: '查看 {count} 个账号',
      hideAccounts: '收起账号',
      myAccount: '我的账号',
      otherAccount: '其他账号',
      until: '{time} 恢复',
      createKey: '用这个分组创建密钥',
      manage: '管理我的账号',
      empty: '没有符合条件的分组',
      states: {
        available: '可用',
        rate_limited: '限流中',
        overloaded: '过载',
        cooling: '临时冷却',
        quota_exceeded: '额度用尽',
        expired: '已过期',
        paused: '已暂停调度',
        error: '错误',
        disabled: '已停用'
      }
    },

    groupOption: {
      private: '私用 · {name}',
      shared: '共享 · {name} · 分成 {rate}%'
    },

    saved: '已保存',
    saveFailed: '保存失败',
    loadFailed: '加载失败',
    deleted: '已删除',
    deleteFailed: '删除失败',
    deleteAccountTitle: '删除上游账号',
    deleteAccountMessage: '删除后该账号不再接收任何请求（包括共享），历史收益保留。确认删除？',
    deleteGroupTitle: '删除私有组',
    deleteGroupMessage: '删除后，仍绑定该组的 API 密钥将无法调用。确认删除？',

    // Shared with admin GroupsView
    supplierRate: '提供者分成比例（0–1）',
    supplierRateHint: '仅对公共余额组内用户提供的账号生效，0.9 表示分成 90%。'
  }
}
