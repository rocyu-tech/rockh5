'use client';

// Password reset page — 2-step flow.
//
// Step 1 (no ?token= in URL): user enters their email → backend generates
//   a 64-char hex token (15min TTL) and "sends" it (in dev, logged to the
//   account-node stdout; in prod, emailed as a link to this same page with
//   ?token=XXX).
//
// Step 2 (?token=XXX in URL): user enters a new password → backend verifies
//   the token, sets the new password, deletes the token, and force-logs-out
//   all existing sessions for that user (via force_logout marker in Redis).
//
// For dev testing without SMTP: tail the account-node logs for a line like
//   [PasswordReset] token generated for user=N (TTL=15m0s)
// then manually visit /forgot-password?token=<64-hex-string>.

import { useState, useEffect, Suspense } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { ArrowLeft, Mail, Lock, CheckCircle2, AlertCircle, Loader2, KeyRound } from 'lucide-react';
import { authApi } from '@/lib/api';
import { useTranslations } from 'next-intl';
import { useLocale } from '@/i18n/provider';

function ForgotPasswordInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const t = useTranslations();
  const { locale } = useLocale();

  const tokenFromUrl = searchParams.get('token') || '';

  const [email, setEmail] = useState('');
  const [token, setToken] = useState(tokenFromUrl);
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    if (tokenFromUrl) setToken(tokenFromUrl);
  }, [tokenFromUrl]);

  const handleRequest = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email) return;
    setSubmitting(true);
    setError(null);
    setSuccess(null);
    try {
      await authApi.requestPasswordReset(email);
      setSuccess(t('auth.resetLinkSent'));
    } catch (err) {
      setSuccess(t('auth.resetLinkSent'));
      console.error('[forgot-password] request error:', err);
    } finally {
      setSubmitting(false);
    }
  };

  const handleConfirm = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(null);

    if (newPassword.length < 8 || newPassword.length > 64) {
      setError(t('auth.passwordTooShort'));
      return;
    }
    if (newPassword !== confirmPassword) {
      setError(t('auth.passwordMismatch'));
      return;
    }
    if (!token) {
      setError(t('auth.invalidToken'));
      return;
    }

    setSubmitting(true);
    try {
      await authApi.confirmPasswordReset(token, newPassword);
      setSuccess(t('auth.resetSuccess'));
      setTimeout(() => router.push('/'), 2000);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '';
      if (msg.includes('invalid') || msg.includes('expired') || msg.includes('token')) {
        setError(t('auth.invalidToken'));
      } else {
        setError(msg || t('common.error'));
      }
    } finally {
      setSubmitting(false);
    }
  };

  const inputCls = "w-full bg-[#0a0a1a] border border-[#f5a623]/20 rounded-lg px-4 py-3 text-white placeholder:text-[#8892b0] focus:outline-none focus:border-[#f5a623] transition-colors";

  return (
    <div className="min-h-screen bg-[#0a0a1a] text-white">
      <div className="flex items-center justify-between p-4 border-b border-[#f5a623]/10">
        <button onClick={() => router.back()} className="flex items-center gap-2 text-[#8892b0] hover:text-white transition-colors">
          <ArrowLeft className="w-5 h-5" />
          <span className="text-sm">{t('common.back')}</span>
        </button>
        <span className="text-xs text-[#8892b0] uppercase tracking-wider">
          {locale === 'zh' ? '中文' : 'EN'}
        </span>
      </div>

      <div className="flex flex-col items-center justify-center px-4 py-12 max-w-md mx-auto">
        <div className="w-full">
          <div className="text-center mb-8">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-[#f5a623]/10 mb-4">
              <KeyRound className="w-8 h-8 text-[#f5a623]" />
            </div>
            <h1 className="text-2xl font-bold mb-2">
              {token ? t('auth.resetYourPassword') : t('auth.resetPassword')}
            </h1>
            <p className="text-sm text-[#8892b0]">
              {token ? t('auth.newPassword') : t('auth.resetPasswordDesc')}
            </p>
          </div>

          {success && (
            <div className="mb-4 p-3 rounded-lg bg-green-500/10 border border-green-500/30 flex items-start gap-2">
              <CheckCircle2 className="w-5 h-5 text-green-400 flex-shrink-0 mt-0.5" />
              <p className="text-sm text-green-300">{success}</p>
            </div>
          )}

          {error && (
            <div className="mb-4 p-3 rounded-lg bg-red-500/10 border border-red-500/30 flex items-start gap-2">
              <AlertCircle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
              <p className="text-sm text-red-300">{error}</p>
            </div>
          )}

          {!token && (
            <form onSubmit={handleRequest} className="space-y-4">
              <div>
                <label className="block text-xs text-[#8892b0] mb-1">{t('auth.email')}</label>
                <div className="relative">
                  <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
                  <input
                    type="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="you@example.com"
                    className={`${inputCls} pl-10`}
                    disabled={submitting}
                  />
                </div>
              </div>
              <button
                type="submit"
                disabled={submitting || !email}
                className="w-full py-3 rounded-lg bg-[#f5a623] text-black font-bold hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-opacity flex items-center justify-center gap-2"
              >
                {submitting ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
                {t('auth.sendResetLink')}
              </button>
            </form>
          )}

          {token && (
            <form onSubmit={handleConfirm} className="space-y-4">
              <div>
                <label className="block text-xs text-[#8892b0] mb-1">Reset Token</label>
                <input
                  type="text"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  className={`${inputCls} font-mono text-xs`}
                  disabled={submitting}
                />
              </div>
              <div>
                <label className="block text-xs text-[#8892b0] mb-1">{t('auth.newPassword')}</label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
                  <input
                    type="password"
                    required
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder="••••••••"
                    className={`${inputCls} pl-10`}
                    disabled={submitting}
                    minLength={8}
                    maxLength={64}
                  />
                </div>
                <p className="text-[10px] text-[#8892b0] mt-1">{t('auth.passwordTooShort')}</p>
              </div>
              <div>
                <label className="block text-xs text-[#8892b0] mb-1">{t('auth.confirmPassword')}</label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
                  <input
                    type="password"
                    required
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="••••••••"
                    className={`${inputCls} pl-10`}
                    disabled={submitting}
                  />
                </div>
              </div>
              <button
                type="submit"
                disabled={submitting || !newPassword || !confirmPassword}
                className="w-full py-3 rounded-lg bg-[#f5a623] text-black font-bold hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-opacity flex items-center justify-center gap-2"
              >
                {submitting ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
                {t('common.confirm')}
              </button>
            </form>
          )}

          <div className="text-center mt-6">
            <button
              onClick={() => router.push('/')}
              className="text-sm text-[#8892b0] hover:text-[#f5a623] transition-colors"
            >
              {t('auth.backToLogin')}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function ForgotPasswordPage() {
  return (
    <Suspense fallback={<div className="min-h-screen bg-[#0a0a1a] text-[#8892b0] flex items-center justify-center">Loading…</div>}>
      <ForgotPasswordInner />
    </Suspense>
  );
}
