import type { ReactNode } from 'react';

import { createContext, use, useCallback, useMemo, useState } from 'react';

import type { Lang, MessageKey } from './messages';

import { messages } from './messages';

const LANG_STORAGE_KEY = 'lang';
const DEFAULT_LANG: Lang = 'zh';

interface I18nContextType {
    lang: Lang;
    setLang: (lang: Lang) => void;
    t: (key: MessageKey) => string;
}

function isLang(value: unknown): value is Lang {
    return value === 'en' || value === 'zh';
}

function resolveInitialLang(): Lang {
    if (typeof window === 'undefined') {
        return DEFAULT_LANG;
    }

    try {
        const stored = window.localStorage.getItem(LANG_STORAGE_KEY);

        if (isLang(stored)) {
            return stored;
        }
    } catch {
        // ignore storage access errors
    }

    // No stored preference: fall back to browser language, still defaulting to zh.
    const browserLang = window.navigator.language?.toLowerCase() ?? '';

    if (browserLang.startsWith('en')) {
        return 'en';
    }

    return DEFAULT_LANG;
}

const I18nContext = createContext<I18nContextType | undefined>(undefined);

export function I18nProvider({ children }: { children: ReactNode }) {
    const [lang, setLangState] = useState<Lang>(resolveInitialLang);

    const setLang = useCallback((next: Lang) => {
        setLangState(next);

        try {
            window.localStorage.setItem(LANG_STORAGE_KEY, next);
        } catch {
            // ignore storage access errors
        }
    }, []);

    const t = useCallback(
        (key: MessageKey): string => {
            return messages[lang]?.[key] ?? messages.en[key] ?? key;
        },
        [lang],
    );

    const value = useMemo<I18nContextType>(() => ({ lang, setLang, t }), [lang, setLang, t]);

    return <I18nContext value={value}>{children}</I18nContext>;
}

// Localized product name for use in <title> and other brand surfaces.
// Falls back to the English name when rendered outside an I18nProvider
// (e.g. isolated unit tests) so it never throws.
export function useAppName(): string {
    const context = use(I18nContext);

    return context ? context.t('appName') : messages.en.appName;
}

export function useLang(): { lang: Lang; setLang: (lang: Lang) => void } {
    const { lang, setLang } = useI18n();

    return { lang, setLang };
}

export function useT(): (key: MessageKey) => string {
    return useI18n().t;
}

function useI18n(): I18nContextType {
    const context = use(I18nContext);

    if (context === undefined) {
        throw new Error('useI18n must be used within an I18nProvider');
    }

    return context;
}

export type { Lang, MessageKey };
