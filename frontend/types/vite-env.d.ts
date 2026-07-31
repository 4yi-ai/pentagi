/// <reference types="vite/client" />

interface ImportMeta {
    readonly env: ImportMetaEnv;
}

interface ImportMetaEnv {
    readonly VITE_APP_API_ROOT: string;
    readonly VITE_APP_LOG_LEVEL: 'DEBUG' | 'ERROR' | 'INFO' | 'WARN';
    readonly VITE_APP_SESSION_KEY: string;
    // Optional auto-login credentials for single-tenant deployments behind an external SSO gateway.
    // When both are set at build time, the app silently logs in and hides the internal login page.
    readonly VITE_AUTOLOGIN_EMAIL?: string;
    readonly VITE_AUTOLOGIN_PASSWORD?: string;
}
