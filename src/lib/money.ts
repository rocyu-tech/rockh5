// Money formatting helpers.
//
// Backend stores all money values as int64 "paise" at x1000 scale
// (1 unit = 1000 paise — see pkg/money/amount.go in rockgame repo).
// 1.234 units → 1234 paise internally.
//
// All amounts MUST flow through these helpers on the frontend — never
// divide by 100 or 1000 inline. Centralizing the conversion prevents
// the historical bug where some pages divided by 100 (showing 10x too
// much) while others divided by 1000 (correct).
//
// Examples (assuming money.Amount = 12345 representing 12.345 units):
//   fmtMoney(12345)             → "12.35"      (2 decimals, default currency)
//   fmtMoney(12345, 'USD')       → "$12.35"
//   fmtMoney(12345, 'CNY')       → "CN¥12.35"
//   fmtMoneyInt(12345)           → 12           (integer truncation for display)
//   parseMoney("12.34")          → 12340        (string → int64 for API requests)

// We intentionally AVOID pulling a full ICU/i18n library here because:
//   1. The backend uses money.Amount = int64 with x1000 scale (pkg/money/amount.go).
//   2. Intl.NumberFormat is built into modern browsers/Node and supports
//      150+ currencies without any extra dependency.
//   3. The previous eslint disable for the unused 'recharts'/'react-hook-form'
//      deps should remind us to keep bundle size minimal.

// SCALE: must match pkg/money.Scale (= 3). 1 unit = 1000 sub-units.
export const MONEY_SCALE = 1000;

/**
 * Format an int64 money.Amount (paise at x1000 scale) as a localized
 * currency string with 2 decimal places.
 *
 * @param paise  int64 from backend (e.g. 12345 represents 12.345 units)
 * @param currency  ISO 4217 code (default 'USD')
 *
 * Examples:
 *   fmtMoney(12345)         → "$12.35"
 *   fmtMoney(12345, 'EUR')  → "€12.35"
 *   fmtMoney(0)             → "$0.00"
 *   fmtMoney(-500, 'USD')   → "-$0.50"
 */
export function fmtMoney(paise: number | undefined | null, currency: string = 'USD'): string {
  const v = Number(paise ?? 0);
  if (!Number.isFinite(v)) return `${currency} 0.00`;
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(v / MONEY_SCALE);
  } catch {
    // Invalid currency code — fall back to plain number.
    return (v / MONEY_SCALE).toFixed(2);
  }
}

/**
 * Format money without currency symbol — just the numeric string.
 * Use when the currency is shown separately (e.g. in a column header).
 *
 * @param paise  int64 from backend
 *
 * Examples:
 *   fmtMoneyPlain(12345)  → "12.35"
 *   fmtMoneyPlain(0)      → "0.00"
 */
export function fmtMoneyPlain(paise: number | undefined | null): string {
  const v = Number(paise ?? 0);
  if (!Number.isFinite(v)) return '0.00';
  return (v / MONEY_SCALE).toFixed(2);
}

/**
 * Format money with explicit +/- sign for net win/loss columns.
 *
 * @param paise  int64 (negative for loss, positive for win)
 *
 * Examples:
 *   fmtMoneySigned(5000)   → "+$5.00"
 *   fmtMoneySigned(-5000)  → "-$5.00"
 *   fmtMoneySigned(0)      → "$0.00"
 */
export function fmtMoneySigned(paise: number | undefined | null, currency: string = 'USD'): string {
  const v = Number(paise ?? 0);
  if (v > 0) return `+${fmtMoney(v, currency)}`;
  if (v < 0) return `-${fmtMoney(Math.abs(v), currency)}`;
  return fmtMoney(0, currency);
}

/**
 * Parse a user-entered money string (e.g. "12.34") into int64 paise
 * for API requests. Truncates extra decimals beyond 3 (backend precision).
 *
 * @param input  User-typed string from an <Input> field
 * @returns int64 paise, or 0 if input is invalid
 *
 * Examples:
 *   parseMoney("12.34")   → 12340
 *   parseMoney("0.001")   → 1
 *   parseMoney("abc")     → 0
 */
export function parseMoney(input: string): number {
  const v = parseFloat(input);
  if (!Number.isFinite(v)) return 0;
  return Math.round(v * MONEY_SCALE);
}

/**
 * Truncate (not round) to 2 decimal places for display in compact UIs
 * where 3-decimal precision is not needed (e.g. compact table cells).
 */
export function fmtMoneyCompact(paise: number | undefined | null): string {
  const v = Number(paise ?? 0);
  if (!Number.isFinite(v)) return '0.00';
  // Truncate (not round) to 2 decimals — safer for financial display.
  const truncated = Math.trunc((v / MONEY_SCALE) * 100) / 100;
  return truncated.toFixed(2);
}
