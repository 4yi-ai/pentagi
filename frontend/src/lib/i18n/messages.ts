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
};

export type Lang = 'en' | 'zh';

export const messages: Record<Lang, Record<MessageKey, string>> = {
    en,
    zh,
};
