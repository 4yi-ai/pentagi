export const APP_NAME = '4YI Pentest';

export type RouteParams = Record<string, string | undefined>;

// React 19 hoists <title> into <head> automatically. The child must be a
// single string of text (template literals are fine — they collapse to one
// string at render time). An empty label falls back to the app name alone.
// `appName` is passed in by callers (via useAppName()) so the suffix follows
// the active locale; it defaults to the English APP_NAME for non-React callers.
export const renderTitle = (label: null | string, appName: string = APP_NAME) => (
    <title>{label ? `${label} — ${appName}` : appName}</title>
);
