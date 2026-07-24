// i18n configuration for rockh5.
//
// We use next-intl WITHOUT locale-based routing (no /en/, /zh/ URL prefix).
// The active locale is stored in localStorage and synced to a cookie so the
// server can pick it up during SSR. This keeps existing URLs stable while
// still allowing runtime language switching.
//
// Supported locales: en, zh (extendable to vi, th — backend already supports
// those for /vip/levels).

export const locales = ['en', 'zh'] as const;
export type Locale = (typeof locales)[number];

export const defaultLocale: Locale = 'en';
export const LOCALE_STORAGE_KEY = 'rockgame_locale';
export const LOCALE_COOKIE = 'rockgame_locale';

export function isLocale(value: string | null | undefined): value is Locale {
  return !!value && (locales as readonly string[]).includes(value);
}

export function getInitialLocale(): Locale {
  if (typeof window === 'undefined') return defaultLocale;
  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  if (isLocale(stored)) return stored;
  // Fall back to browser language
  const nav = window.navigator.language?.toLowerCase() ?? '';
  if (nav.startsWith('zh')) return 'zh';
  return defaultLocale;
}
