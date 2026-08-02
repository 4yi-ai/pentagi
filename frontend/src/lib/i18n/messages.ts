// Lightweight, dependency-free i18n dictionary.
// Keys are shared between all languages; `en` defines the canonical key set.

export const en = {
    // Sidebar
    newFlow: 'New Flow',
    dashboard: 'Dashboard',
    flows: 'Flows',
    templates: 'Templates',
    resources: 'Resources',
    knowledges: 'Knowledges',
    recentFlows: 'Recent Flows',
    favoriteFlows: 'Favorite Flows',
    settings: 'Settings',
    theme: 'Theme',
    changePassword: 'Change Password',
    logout: 'Log out',
    language: 'Language',

    // Login form
    login: 'Login',
    password: 'Password',
    signIn: 'Sign in',
    enterYourEmail: 'Enter your email',
    enterYourPassword: 'Enter your password',
    continueWithGoogle: 'Continue with Google',
    continueWithGitHub: 'Continue with GitHub',
    or: 'or',

    // Common
    cancel: 'Cancel',
    save: 'Save',

    // Flows page
    noFlowsFound: 'No flows found',
    getStartedFirstFlow: 'Get started by creating your first conversation flow',
    filterFlows: 'Filter flows...',
    loadingFlows: 'Loading flows...',
    loadingFlowsDescription: 'Please wait while we fetch your conversation flows',

    // Data table
    tableRowsPerPage: 'Rows per page',
    tableShowingRange: 'Showing {start}–{end} of {total}',
    tablePageOf: 'Page {page} of {total}',
    tableFirstPage: 'First page',
    tablePreviousPage: 'Previous page',
    tableNextPage: 'Next page',
    tableLastPage: 'Last page',

    // Flow table columns
    colId: 'ID',
    colTitle: 'Title',
    colStatus: 'Status',
    colProvider: 'Provider',
    colTerminals: 'Terminals',
    colCreated: 'Created',
    colUpdated: 'Updated',
    noTerminals: 'No terminals',

    // Flow status
    statusCreated: 'Created',
    statusWaiting: 'Waiting',
    statusRunning: 'Running',
    statusFinished: 'Finished',
    statusFailed: 'Failed',

    // New flow
    createNewFlow: 'Create a new flow',
    newFlowDescribe: 'Describe what you would like PentAGI to test',
    newFlowPromptPlaceholder: 'Describe what you would like PentAGI to test...',
    newFlowAssistantPlaceholder: 'What would you like me to help you with?',
    newFlowCreatingPlaceholder: 'Creating a new flow...',

    // Flow tabs
    tabAutomation: 'Automation',
    tabAssistant: 'Assistant',
    tabTerminal: 'Terminal',
    tabTasks: 'Tasks',
    tabAgents: 'Agents',
    tabSearches: 'Searches',
    tabVectorStore: 'Vector Store',
    tabFiles: 'Files',
    tabScreenshots: 'Screenshots',

    // Settings
    settingsProviders: 'Providers',
    settingsPrompts: 'Prompts',
    settingsApiTokens: 'API Tokens',
    settingsCreateProvider: 'Create Provider',
    settingsEditProvider: 'Edit Provider',
    settingsCreatePrompt: 'Create Prompt',
    settingsEditPrompt: 'Edit Prompt',
    settingsAddProvider: 'Add Provider',
    settingsCreateToken: 'Create Token',
    backToApp: 'Back to App',
} as const;

export type MessageKey = keyof typeof en;

export const zh: Record<MessageKey, string> = {
    // Sidebar
    newFlow: '新建流程',
    dashboard: '仪表盘',
    flows: '流程',
    templates: '模板',
    resources: '资源',
    knowledges: '知识库',
    recentFlows: '最近流程',
    favoriteFlows: '收藏的流程',
    settings: '设置',
    theme: '主题',
    changePassword: '修改密码',
    logout: '退出登录',
    language: '语言',

    // Login form
    login: '登录',
    password: '密码',
    signIn: '登录',
    enterYourEmail: '请输入邮箱',
    enterYourPassword: '请输入密码',
    continueWithGoogle: '使用 Google 登录',
    continueWithGitHub: '使用 GitHub 登录',
    or: '或',

    // Common
    cancel: '取消',
    save: '保存',

    // Flows page
    noFlowsFound: '暂无流程',
    getStartedFirstFlow: '创建你的第一个流程开始使用',
    filterFlows: '筛选流程…',
    loadingFlows: '正在加载流程…',
    loadingFlowsDescription: '正在获取你的流程，请稍候',

    // Data table
    tableRowsPerPage: '每页行数',
    tableShowingRange: '显示第 {start}–{end} 条,共 {total} 条',
    tablePageOf: '第 {page} 页,共 {total} 页',
    tableFirstPage: '首页',
    tablePreviousPage: '上一页',
    tableNextPage: '下一页',
    tableLastPage: '末页',

    // Flow table columns
    colId: 'ID',
    colTitle: '标题',
    colStatus: '状态',
    colProvider: '提供方',
    colTerminals: '终端',
    colCreated: '创建时间',
    colUpdated: '更新时间',
    noTerminals: '无终端',

    // Flow status
    statusCreated: '已创建',
    statusWaiting: '等待中',
    statusRunning: '运行中',
    statusFinished: '已完成',
    statusFailed: '失败',

    // New flow
    createNewFlow: '创建新流程',
    newFlowDescribe: '描述你想让 PentAGI 测试的内容',
    newFlowPromptPlaceholder: '描述你想让 PentAGI 测试的内容…',
    newFlowAssistantPlaceholder: '有什么可以帮你的吗？',
    newFlowCreatingPlaceholder: '正在创建新流程…',

    // Flow tabs
    tabAutomation: '自动化',
    tabAssistant: '助手',
    tabTerminal: '终端',
    tabTasks: '任务',
    tabAgents: '智能体',
    tabSearches: '搜索',
    tabVectorStore: '向量库',
    tabFiles: '文件',
    tabScreenshots: '截图',

    // Settings
    settingsProviders: '提供方',
    settingsPrompts: '提示词',
    settingsApiTokens: 'API 令牌',
    settingsCreateProvider: '创建提供方',
    settingsEditProvider: '编辑提供方',
    settingsCreatePrompt: '创建提示词',
    settingsEditPrompt: '编辑提示词',
    settingsAddProvider: '添加提供方',
    settingsCreateToken: '创建令牌',
    backToApp: '返回应用',
};

export type Lang = 'en' | 'zh';

export const messages: Record<Lang, Record<MessageKey, string>> = {
    en,
    zh,
};
