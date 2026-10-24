export default {
  relay: {
    title: 'My relay',
    description: 'Connect your own upstream accounts (API key or OAuth login) to this site. Use them yourself for free, or share them with other users to earn spending credit.',
    viewStatus: 'View all group status',
    refresh: 'Refresh',

    modes: {
      private: {
        title: 'Use it yourself',
        tag: 'Free',
        p1: 'Only you can use it, through API keys bound to a private group',
        p2: 'No site charge and no use of your paid quota',
        p3: 'Upstream costs, quotas, concurrency and rate limits still apply'
      },
      shared: {
        title: 'Share and earn',
        tag: 'Optional',
        p1: 'Tick public groups on an account and other users of those groups may be routed to it',
        p2: 'You earn a share of each call as spending credit (no withdrawals)',
        p3: 'Your upstream credentials are never exposed; sharing and self-use draw on the same upstream quota'
      }
    },

    guide: {
      title: 'Get started in three steps',
      show: 'Show setup steps',
      hide: 'Hide',
      done: 'Done',
      step1Title: 'Connect an upstream',
      step1Desc: 'Add an upstream API key or sign in with OAuth, and put the account in a private group.',
      step1Action: 'Connect upstream',
      step2Title: 'Create an API key',
      step2Desc: 'On the API keys page, create a key and choose your private group:',
      step2Action: 'Create a key',
      step3Title: 'Call this site',
      step3Desc: 'Point your client at this site’s API address with the new key. Requests go to your own upstream.',
      step3Fallback: 'Use this site’s API address (the same as any other key)',
      copy: 'Copy',
      copied: 'Copied'
    },

    tabs: {
      accounts: 'Upstream accounts',
      groups: 'Private groups',
      earnings: 'Sharing earnings'
    },

    accounts: {
      add: 'Connect upstream',
      emptyTitle: 'No upstream accounts yet',
      emptyDesc: 'Once connected, use it for free through a private group, or share it to earn credit.',
      account: 'Account',
      usage: 'Used for',
      status: 'Status',
      actions: 'Actions',
      privateIn: 'Private',
      sharedIn: 'Shared',
      notInGroup: 'Not in any group, so it will not receive calls',
      test: 'Test',
      stats: 'Stats',
      reauth: 'Re-authorize',
      refreshToken: 'Refresh token',
      publicTitle: 'Public groups you can share to',
      publicDesc: 'Edit an account and tick these groups to start sharing; untick to stop. The account platform must match the group platform.',
      publicEmpty: 'No public groups are open for sharing yet.',
      share: '{rate}% share',
      price: '{rate}x price'
    },

    groups: {
      intro: 'A private group decides which accounts your API keys call. A key bound to a private group only uses your own accounts in it, with no site charge. The default group is enough for most people; create more only to separate usage (e.g. work and personal).',
      add: 'New private group',
      edit: 'Edit private group',
      createKey: 'Create key for this group',
      accountCount: '{count} accounts',
      noAccounts: 'No accounts in this group yet',
      defaultName: 'Default private group',
      composite: 'Mixed (any platform)',
      platformHint: 'A mixed group accepts accounts from any platform; a platform group accepts only that platform. This cannot be changed later.',
      statusHint: 'When disabled, keys bound to this group cannot make calls.',
      deleteHint: 'After deletion, keys still bound to this group are rejected.'
    },

    earnings: {
      total: 'Total earnings',
      today: 'Today (UTC)',
      pending: 'Pending',
      intro: 'Earnings only come from calls through public groups that land on your shared accounts (including your own calls). Private-group self-use earns nothing and costs nothing.',
      pendingNotice: '{amount} is pending: when the caller’s balance is short, earnings are recorded first and credited after they top up.',
      emptyTitle: 'No earnings yet',
      emptyDesc: 'Tick public groups on an account to start sharing. Earnings appear here when it is called.',
      time: 'Time',
      account: 'Account',
      group: 'Public group',
      charged: 'Caller paid',
      earned: 'You earned',
      selfUse: 'Self-use',
      pendingTag: 'Pending'
    },

    form: {
      name: 'Name',
      platform: 'Platform',
      status: 'Status',
      description: 'Description (optional)',
      active: 'Active',
      disabled: 'Disabled'
    },

    status: {
      title: 'Group status',
      description: 'Every group you can call, with the live state of each account in it. A key bound to a group is only routed to that group’s available accounts; if none are available, requests fail.',
      refresh: 'Refresh',
      autoRefresh: 'Auto-refreshes every 30 seconds',
      search: 'Search groups',
      onlyProblems: 'Only groups with problems',
      filters: { all: 'All', private: 'My private groups', public_relay: 'Public shared groups', standard: 'Platform groups' },
      kinds: { private: 'Private', public_relay: 'Public shared', standard: 'Platform' },
      kindHints: {
        private: 'Only you can use it; calls only your own accounts at no site charge',
        public_relay: 'Charged at group price; other users can share accounts into it',
        standard: 'Accounts provided by the administrator; charged at group price'
      },
      summary: { groups: 'Callable groups', healthy: 'Healthy', degraded: 'Degraded', down: 'No available account', empty: 'No accounts' },
      health: {
        healthy: 'All accounts available',
        degraded: 'Some accounts unavailable',
        down: 'No account is available right now; requests will fail',
        empty: 'This group has no accounts yet; requests will fail'
      },
      available: '{available} of {total} available',
      mineCount: '{count} of them yours',
      free: 'Free',
      price: '{rate}x price',
      subscription: 'Subscription',
      share: '{rate}% supplier share',
      groupDisabled: 'Group disabled',
      showAccounts: 'Show {count} accounts',
      hideAccounts: 'Hide accounts',
      myAccount: 'My account',
      otherAccount: 'Other account',
      until: 'recovers {time}',
      createKey: 'Create a key for this group',
      manage: 'Manage my accounts',
      empty: 'No groups match',
      states: {
        available: 'Available',
        rate_limited: 'Rate limited',
        overloaded: 'Overloaded',
        cooling: 'Cooling down',
        quota_exceeded: 'Quota used up',
        expired: 'Expired',
        paused: 'Scheduling paused',
        error: 'Error',
        disabled: 'Disabled'
      }
    },

    groupOption: {
      private: 'Private · {name}',
      shared: 'Shared · {name} · {rate}% share'
    },

    saved: 'Saved',
    saveFailed: 'Could not save',
    loadFailed: 'Could not load',
    deleted: 'Deleted',
    deleteFailed: 'Could not delete',
    deleteAccountTitle: 'Delete upstream account',
    deleteAccountMessage: 'The account will stop receiving all calls, including shared ones. Historical earnings remain. Delete it?',
    deleteGroupTitle: 'Delete private group',
    deleteGroupMessage: 'API keys still bound to this group will stop working. Delete it?',

    // Shared with admin GroupsView
    supplierRate: 'Supplier share (0–1)',
    supplierRateHint: 'Applies to user-owned accounts in public balance groups. 0.9 means a 90% share.'
  }
}
