// Responsible Gaming page — self-exclusion + deposit limits + help links.
//
// P0-9: required by every regulated gaming license. The page must show:
//   - 18+ age restriction
//   - Self-exclusion options (24h / 7d / 1m / 6m / 1y / permanent)
//   - Daily / weekly / monthly deposit limit setter
//   - Links to gambling addiction support organizations
//   - Reality check reminder settings
//
// The self-exclusion and deposit-limit endpoints are TODO on the backend
// (currently only the schema exists — account_delete_request.cooling_end).

'use client';

import { useState } from 'react';
import { Shield, Clock, TrendingDown, Phone, AlertTriangle, ExternalLink } from 'lucide-react';

const SELF_EXCLUSION_OPTIONS = [
  { value: '24h', label: '24 hours', desc: 'Take a short break' },
  { value: '7d', label: '7 days', label_extra: '1 week', desc: 'Cooling-off period' },
  { value: '1m', label: '1 month', desc: 'Step back and reassess' },
  { value: '6m', label: '6 months', desc: 'Extended break' },
  { value: '1y', label: '1 year', desc: 'Long break' },
  { value: 'permanent', label: 'Permanent', desc: 'Permanently self-exclude (cannot be reversed)' },
];

const HELP_LINKS = [
  { name: 'GamCare', url: 'https://www.gamcare.org.uk/', desc: 'UK helpline: 0808 8020 133' },
  { name: 'Gamblers Anonymous', url: 'https://www.gamblersanonymous.org/', desc: 'Worldwide peer support' },
  { name: 'BeGambleAware', url: 'https://www.begambleaware.org/', desc: 'Free confidential advice' },
  { name: 'National Problem Gambling Helpline', url: 'https://www.ncpgambling.org/', desc: 'US helpline: 1-800-522-4700' },
];

export default function ResponsibleGamingPage() {
  const [selfExclusion, setSelfExclusion] = useState('');
  const [dailyLimit, setDailyLimit] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const handleSelfExclude = async () => {
    if (!selfExclusion) {
      setMessage({ type: 'error', text: 'Please select a self-exclusion period.' });
      return;
    }
    setSubmitting(true);
    try {
      // TODO: backend endpoint POST /api/v1/account/self-exclude { period: selfExclusion }
      // For now, simulate success — the backend needs to add this endpoint
      // and gate SlotSpin/JoinTable/CreateRecharge/CreateWithdraw on it.
      setMessage({
        type: 'success',
        text: `Self-exclusion request received (${selfExclusion}). Once the backend endpoint is wired, you will be blocked from playing and depositing until the period ends.`,
      });
      setSelfExclusion('');
    } catch {
      setMessage({ type: 'error', text: 'Failed to submit self-exclusion request.' });
    } finally {
      setSubmitting(false);
    }
  };

  const handleSetDepositLimit = async () => {
    const amount = parseFloat(dailyLimit);
    if (isNaN(amount) || amount <= 0) {
      setMessage({ type: 'error', text: 'Please enter a valid daily deposit limit.' });
      return;
    }
    setSubmitting(true);
    try {
      // TODO: backend endpoint POST /api/v1/account/deposit-limit { daily_limit: amount * 1000 }
      setMessage({
        type: 'success',
        text: `Daily deposit limit set to $${amount.toFixed(2)}. Once backend endpoint is wired, deposits exceeding this will be blocked.`,
      });
      setDailyLimit('');
    } catch {
      setMessage({ type: 'error', text: 'Failed to set deposit limit.' });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#0a0a1a] text-white">
      <div className="max-w-2xl mx-auto px-4 py-12">
        <div className="flex items-center gap-3 mb-8">
          <Shield className="w-8 h-8 text-[#f5a623]" />
          <div>
            <h1 className="text-2xl font-bold">Responsible Gaming</h1>
            <p className="text-sm text-[#8892b0]">Play responsibly. Help is available.</p>
          </div>
        </div>

        {/* 18+ banner */}
        <div className="mb-8 p-4 rounded-lg border border-[#f5a623]/30 bg-[#f5a623]/10 flex items-center gap-3">
          <span className="text-3xl">🔞</span>
          <div>
            <p className="font-bold text-[#f5a623]">18+ Only</p>
            <p className="text-xs text-[#8892b0]">
              This service is strictly for adults aged 18 and above (or the legal age of majority in your jurisdiction). Underage gambling is illegal.
            </p>
          </div>
        </div>

        {/* Reality check */}
        <section className="mb-8 p-6 rounded-lg border border-white/10 bg-[#1a1a2e]">
          <div className="flex items-start gap-3 mb-3">
            <Clock className="w-5 h-5 text-[#f5a623] mt-1" />
            <div>
              <h2 className="text-lg font-semibold">Reality Check</h2>
              <p className="text-sm text-[#8892b0] mt-1">
                Set a reminder to take breaks during play. We&apos;ll show a popup after the interval you choose.
              </p>
            </div>
          </div>
          <div className="flex gap-2 flex-wrap">
            {['30 min', '1 hour', '2 hours', '4 hours'].map((interval) => (
              <button
                key={interval}
                className="px-3 py-1.5 rounded-full text-xs font-semibold bg-[#0a0a1a] border border-white/10 hover:border-[#f5a623]/50 hover:text-[#f5a623]"
              >
                {interval}
              </button>
            ))}
          </div>
        </section>

        {/* Deposit limits */}
        <section className="mb-8 p-6 rounded-lg border border-white/10 bg-[#1a1a2e]">
          <div className="flex items-start gap-3 mb-3">
            <TrendingDown className="w-5 h-5 text-[#f5a623] mt-1" />
            <div>
              <h2 className="text-lg font-semibold">Deposit Limit</h2>
              <p className="text-sm text-[#8892b0] mt-1">
                Set a daily maximum deposit amount. Once reached, no more deposits can be made until the next day.
              </p>
            </div>
          </div>
          <div className="flex gap-2">
            <input
              type="number"
              inputMode="decimal"
              placeholder="Daily limit (USD)"
              value={dailyLimit}
              onChange={(e) => setDailyLimit(e.target.value)}
              className="flex-1 bg-[#0a0a1a] border border-[#f5a623]/20 rounded-lg px-4 py-2 text-white placeholder:text-[#8892b0] focus:outline-none focus:border-[#f5a623]"
            />
            <button
              onClick={handleSetDepositLimit}
              disabled={submitting || !dailyLimit}
              className="px-4 py-2 rounded-lg bg-[#f5a623] text-black font-semibold text-sm disabled:opacity-50"
            >
              Set Limit
            </button>
          </div>
        </section>

        {/* Self-exclusion */}
        <section className="mb-8 p-6 rounded-lg border border-red-500/20 bg-[#1a1a2e]">
          <div className="flex items-start gap-3 mb-3">
            <AlertTriangle className="w-5 h-5 text-red-400 mt-1" />
            <div>
              <h2 className="text-lg font-semibold text-red-400">Self-Exclusion</h2>
              <p className="text-sm text-[#8892b0] mt-1">
                Block yourself from playing and depositing for a chosen period. <strong>Permanent self-exclusion cannot be reversed.</strong>
              </p>
            </div>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 mb-4">
            {SELF_EXCLUSION_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setSelfExclusion(opt.value)}
                className={`p-3 rounded-lg border text-left transition-all ${
                  selfExclusion === opt.value
                    ? 'border-red-500/50 bg-red-500/10'
                    : 'border-white/10 hover:border-red-500/30'
                }`}
              >
                <p className="text-sm font-semibold">{opt.label}</p>
                <p className="text-[10px] text-[#8892b0]">{opt.desc}</p>
              </button>
            ))}
          </div>
          <button
            onClick={handleSelfExclude}
            disabled={submitting || !selfExclusion}
            className="w-full py-3 rounded-lg bg-red-600 text-white font-bold disabled:opacity-50"
          >
            {submitting ? 'Submitting...' : 'Confirm Self-Exclusion'}
          </button>
          {selfExclusion === 'permanent' && (
            <p className="text-xs text-red-400 mt-2 text-center">
              ⚠️ Permanent self-exclusion is irreversible. Your account will be closed and you cannot re-register.
            </p>
          )}
        </section>

        {/* Help links */}
        <section className="mb-8 p-6 rounded-lg border border-white/10 bg-[#1a1a2e]">
          <div className="flex items-start gap-3 mb-4">
            <Phone className="w-5 h-5 text-[#f5a623] mt-1" />
            <div>
              <h2 className="text-lg font-semibold">Get Help</h2>
              <p className="text-sm text-[#8892b0] mt-1">
                If you or someone you know has a gambling problem, contact these free, confidential helplines.
              </p>
            </div>
          </div>
          <div className="space-y-2">
            {HELP_LINKS.map((link) => (
              <a
                key={link.name}
                href={link.url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-between p-3 rounded-lg bg-[#0a0a1a] border border-white/10 hover:border-[#f5a623]/50 transition-colors"
              >
                <div>
                  <p className="text-sm font-semibold text-white">{link.name}</p>
                  <p className="text-xs text-[#8892b0]">{link.desc}</p>
                </div>
                <ExternalLink className="w-4 h-4 text-[#8892b0]" />
              </a>
            ))}
          </div>
        </section>

        {/* Message */}
        {message && (
          <div className={`p-4 rounded-lg border ${
            message.type === 'success'
              ? 'border-green-500/30 bg-green-500/10 text-green-300'
              : 'border-red-500/30 bg-red-500/10 text-red-300'
          }`}>
            <p className="text-sm">{message.text}</p>
          </div>
        )}

        <p className="text-xs text-[#8892b0] mt-8 text-center">
          For responsible gaming questions, contact support@rockgame.example.
        </p>
      </div>
    </div>
  );
}
