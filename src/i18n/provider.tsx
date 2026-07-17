'use client';

// React context + hook for runtime locale switching without locale-based routing.
//
// Usage:
//   const t = useTranslations();
//   t('nav.home')  // → "Home" or "首页"
//
//   const { locale, setLocale } = useLocale();
//   setLocale('zh');

import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { IntlProvider } from 'next-intl';
import {
  LOCALE_COOKIE,
  LOCALE_STORAGE_KEY,
  type Locale,
  defaultLocale,
  getInitialLocale,
  isLocale,
  locales,
} from './config';

import en from './messages/en.json';
import zh from './messages/zh.json';

const messages: Record<Locale, Record<string, string>> = {
  en: en as Record<string, string>,
  zh: zh as Record<string, string>,
};

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
}

const I18nContext = createContext<I18nContextValue>({
  locale: defaultLocale,
  setLocale: () => {},
});

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(defaultLocale);

  // Hydrate from localStorage / browser language on mount (client-only).
  useEffect(() => {
    setLocaleState(getInitialLocale());
  }, []);

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(LOCALE_STORAGE_KEY, next);
      // Cookie so the server can pick up the locale for SSR / future requests
      // (currently informational — backend doesn't act on it except for /vip/levels).
      document.cookie = `${LOCALE_COOKIE}=${next}; path=/; max-age=${60 * 60 * 24 * 365}; SameSite=Lax`;
    }
  }, []);

  const value = useMemo(() => ({ locale, setLocale }), [locale, setLocale]);

  return (
    <I18nContext.Provider value={value}>
      <IntlProvider
        locale={locale}
        messages={messages[locale]}
        getMessageFallback={(info) => {
          // During SSR (before this provider hydrates with the chosen locale),
          // next-intl can throw MISSING_MESSAGE. Return the key as a fallback
          // so prerendering doesn't crash; the client provider will replace
          // it with the real translation after hydration.
          return info.key;
        }}
        onError={(err) => {
          // Swallow MISSING_MESSAGE errors at runtime — they're transient
          // (provider hasn't hydrated yet) and the fallback above handles
          // the visual gap.
          if (err.code === 'MISSING_MESSAGE') return;
          console.error('[i18n]', err);
        }}
      >
        {children}
      </IntlProvider>
    </I18nContext.Provider>
  );
}

export function useLocale() {
  return useContext(I18nContext);
}

export { locales };
