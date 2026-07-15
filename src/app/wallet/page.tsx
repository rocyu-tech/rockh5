'use client';

import { useState, useEffect } from 'react';
import { useSearchParams } from 'next/navigation';
import { useAuthStore } from '@/store/auth';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';
import { shopApi } from '@/lib/api';
import Navbar from '@/components/Navbar';
import {
  Wallet, CreditCard, Building, Smartphone, Zap,
  Loader2, Check, Copy, ExternalLink, AlertTriangle,
  ArrowDownToLine, ArrowUpFromLine,
} from 'lucide-react';

// === Shared types ===
interface Channel {
  id: number;
  name: string;
  icon?: string;
  min_amount: number;
  max_amount: number;
  type?: string;
}

interface WithdrawChannel extends Channel {
  daily_limit: number;
  need_account: boolean;
}

const PRESET_AMOUNTS = [10, 20, 50, 100, 200, 500, 1000, 2000];

const CHANNEL_ICONS: Record<string, typeof CreditCard> = {
  usdt: CreditCard,
  crypto: Zap,
  bank: Building,
  upi: Smartphone,
  trc20: Zap,
  erc20: CreditCard,
};

// === Deposit Tab Component ===
function DepositTab({ onGoBack }: { onGoBack: () => void }) {
  const { assets, fetchAssets } = useAuthStore();
  const apiStatus = useApiStatusContext();

  const [channels, setChannels] = useState<Channel[]>([]);
  const [selectedChannel, setSelectedChannel] = useState<number | null>(null);
  const [amount, setAmount] = useState('');
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [payUrl, setPayUrl] = useState('');
  const [orderNo, setOrderNo] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(true);
    shopApi.getPaymentChannels().then((res) => {
      const data = res.data?.data;
      if (Array.isArray(data)) {
        const list = data as Channel[];
        setChannels(list);
        if (list.length > 0) setSelectedChannel(list[0].id);
      }
    }).catch((err) => {
      apiStatus.markFailed('shop/payment-channels', getErrorMessage(err));
    }).finally(() => setLoading(false));
  }, []);

  const selectedChannelData = channels.find((c) => c.id === selectedChannel);

  const handleSubmit = async () => {
    if (!selectedChannel || !amount) {
      setError('Please select a payment method and enter amount');
      return;
    }
    const numAmount = parseFloat(amount);
    if (isNaN(numAmount) || numAmount <= 0) {
      setError('Please enter a valid amount');
      return;
    }
    if (selectedChannelData && numAmount < selectedChannelData.min_amount) {
      setError(`Minimum amount is $${selectedChannelData.min_amount}`);
      return;
    }
    if (selectedChannelData && numAmount > selectedChannelData.max_amount) {
      setError(`Maximum amount is $${selectedChannelData.max_amount}`);
      return;
    }

    setError('');
    setSubmitting(true);
    try {
      const res = await shopApi.recharge({ channel_id: selectedChannel, amount: numAmount });
      const data = res.data?.data;
      if (data?.pay_url) {
        setPayUrl(data.pay_url);
        setOrderNo(data.order_no || '');
      } else {
        setError('Payment URL not available');
      }
    } catch (err) {
      setError(getErrorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  // Payment redirect screen
  if (payUrl) {
    return (
      <div>
        <div className="flex items-center gap-2 mb-4">
          <ArrowDownToLine className="w-5 h-5 text-[#f5a623]" />
          <h2 className="text-sm font-bold text-white">Deposit</h2>
        </div>

        <div className="rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/20 p-6 text-center">
          <div className="w-14 h-14 rounded-full bg-green-500/10 flex items-center justify-center mx-auto mb-4">
            <Check className="w-7 h-7 text-green-400" />
          </div>
          <h3 className="text-lg font-bold text-white mb-2">Order Created</h3>
          <p className="text-xs text-[#8892b0] mb-1">Order No: {orderNo}</p>
          <p className="text-sm text-[#8892b0] mb-4">Redirecting to payment page...</p>
          <div className="flex gap-3 justify-center">
            <button
              onClick={() => window.open(payUrl, '_blank')}
              className="px-5 py-2.5 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold rounded-lg text-sm flex items-center gap-2 active:scale-95 transition-transform"
            >
              <ExternalLink className="w-4 h-4" />
              Open Payment
            </button>
          </div>
          <button
            onClick={() => {
              navigator.clipboard?.writeText(payUrl);
            }}
            className="mt-3 text-xs text-[#8892b0] flex items-center gap-1 mx-auto active:text-[#f5a623]"
          >
            <Copy className="w-3.5 h-3.5" />
            Copy Payment Link
          </button>
          <button
            onClick={() => { setPayUrl(''); setOrderNo(''); fetchAssets(); }}
            className="mt-4 text-xs text-[#f5a623] font-medium active:opacity-70"
          >
            Back to Deposit
          </button>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <ArrowDownToLine className="w-5 h-5 text-[#f5a623]" />
        <h2 className="text-sm font-bold text-white">Deposit</h2>
      </div>

      {/* Payment Channels */}
      <div className="mb-4">
        <h3 className="text-xs font-semibold text-[#8892b0] mb-2">Payment Method</h3>
        {loading ? (
          <div className="flex items-center justify-center py-6">
            <Loader2 className="w-4 h-4 text-[#f5a623] animate-spin" />
          </div>
        ) : channels.length === 0 ? (
          <p className="text-xs text-[#8892b0] py-4 text-center">No payment channels available</p>
        ) : (
          <div className="grid grid-cols-3 gap-2">
            {channels.map((channel) => {
              const Icon = CHANNEL_ICONS[channel.name?.toLowerCase()] || CreditCard;
              const isSelected = selectedChannel === channel.id;
              return (
                <button
                  key={channel.id}
                  onClick={() => setSelectedChannel(channel.id)}
                  className={`flex flex-col items-center gap-1.5 p-2.5 rounded-xl border transition-all active:scale-95 ${
                    isSelected
                      ? 'bg-[#f5a623]/10 border-[#f5a623]/40'
                      : 'bg-[#1a1a2e]/60 border-[#f5a623]/10'
                  }`}
                >
                  <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${
                    isSelected ? 'bg-[#f5a623]/20' : 'bg-[#1a1a2e]'
                  }`}>
                    <Icon className={`w-4 h-4 ${isSelected ? 'text-[#f5a623]' : 'text-[#8892b0]'}`} />
                  </div>
                  <span className={`text-[10px] font-medium ${isSelected ? 'text-[#f5a623]' : 'text-[#ccd6f6]'}`}>
                    {channel.name}
                  </span>
                </button>
              );
            })}
          </div>
        )}
      </div>

      {/* Amount input */}
      <div className="mb-3">
        <h3 className="text-xs font-semibold text-[#8892b0] mb-2">Amount</h3>
        <div className="flex items-center bg-[#16213e] border border-[#f5a623]/20 rounded-xl overflow-hidden focus-within:border-[#f5a623]/50 transition-colors">
          <span className="pl-4 text-lg text-[#f5a623] font-bold">$</span>
          <input
            type="number"
            inputMode="decimal"
            placeholder="0.00"
            value={amount}
            onChange={(e) => { setAmount(e.target.value); setError(''); }}
            className="flex-1 bg-transparent text-white text-xl font-semibold px-2 py-3 outline-none placeholder-[#8892b0]/40 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
          />
        </div>
        {selectedChannelData && (
          <p className="text-[10px] text-[#8892b0] mt-1">
            Min: ${selectedChannelData.min_amount} ~ Max: ${selectedChannelData.max_amount}
          </p>
        )}
      </div>

      {/* Preset amounts */}
      <div className="mb-3">
        <div className="grid grid-cols-4 gap-2">
          {PRESET_AMOUNTS.map((val) => (
            <button
              key={val}
              onClick={() => { setAmount(String(val)); setError(''); }}
              className={`py-2 rounded-xl text-xs font-semibold transition-all active:scale-95 ${
                amount === String(val)
                  ? 'bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] shadow-lg shadow-[#f5a623]/20'
                  : 'bg-[#1a1a2e] text-[#ccd6f6] border border-[#f5a623]/10'
              }`}
            >
              ${val}
            </button>
          ))}
        </div>
      </div>

      {/* Error */}
      {error && (
        <p className="text-xs text-[#e94560] bg-[#e94560]/10 px-3 py-2 rounded-lg mb-3">{error}</p>
      )}

      {/* Submit */}
      <button
        onClick={handleSubmit}
        disabled={submitting || !selectedChannel || !amount}
        className="w-full py-3 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold rounded-xl text-sm shadow-lg shadow-[#f5a623]/20 disabled:opacity-40 active:scale-[0.98] transition-all"
      >
        {submitting ? (
          <span className="flex items-center justify-center gap-2">
            <Loader2 className="w-4 h-4 animate-spin" />
            Processing...
          </span>
        ) : (
          'Deposit Now'
        )}
      </button>
    </div>
  );
}

// === Withdraw Tab Component ===
function WithdrawTab({ onGoBack }: { onGoBack: () => void }) {
  const { assets, fetchAssets } = useAuthStore();

  const [channels, setChannels] = useState<WithdrawChannel[]>([]);
  const [selectedChannel, setSelectedChannel] = useState<number | null>(null);
  const [amount, setAmount] = useState('');
  const [account, setAccount] = useState('');
  const [accountName, setAccountName] = useState('');
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [orderNo, setOrderNo] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(true);
    shopApi.getWithdrawChannels().then((res) => {
      const data = res.data?.data;
      if (Array.isArray(data)) {
        const list = data as WithdrawChannel[];
        setChannels(list);
        if (list.length > 0) setSelectedChannel(list[0].id);
      }
    }).catch(() => {
      // BG-6 FIX: removed hardcoded mock withdraw channels;
      // on failure, channels stays empty and the UI shows a proper empty state.
      setChannels([]);
      setSelectedChannel(null);
    }).finally(() => setLoading(false));
  }, []);

  const selectedChannelData = channels.find((c) => c.id === selectedChannel);
  const balance = assets?.balance ?? 0;

  const handleAmountClick = (val: number) => {
    if (val > balance) { setError('Insufficient balance'); return; }
    setAmount(String(val));
    setError('');
  };

  const handleSubmit = async () => {
    if (!selectedChannel || !amount) {
      setError('Please select a withdrawal method and enter amount');
      return;
    }
    const numAmount = parseFloat(amount);
    if (isNaN(numAmount) || numAmount <= 0) {
      setError('Please enter a valid amount');
      return;
    }
    if (numAmount > balance) {
      setError('Insufficient balance');
      return;
    }
    if (selectedChannelData) {
      if (numAmount < selectedChannelData.min_amount) {
        setError(`Minimum withdrawal is $${selectedChannelData.min_amount}`);
        return;
      }
      if (numAmount > selectedChannelData.max_amount) {
        setError(`Maximum withdrawal is $${selectedChannelData.max_amount}`);
        return;
      }
      if (selectedChannelData.need_account && !account.trim()) {
        setError('Please enter your withdrawal address / account number');
        return;
      }
    }

    setError('');
    setSubmitting(true);
    try {
      const res = await shopApi.withdraw({
        channel_id: selectedChannel,
        amount: numAmount,
        account: account.trim() || undefined,
      });
      const data = res.data?.data;
      if (data?.order_no) {
        setOrderNo(data.order_no);
        setAmount('');
        setAccount('');
        setAccountName('');
        fetchAssets();
      } else {
        setError('Withdrawal request failed, please try again');
      }
    } catch (err) {
      setError(getErrorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  // Success screen
  if (orderNo) {
    return (
      <div>
        <div className="flex items-center gap-2 mb-4">
          <ArrowUpFromLine className="w-5 h-5 text-[#e94560]" />
          <h2 className="text-sm font-bold text-white">Withdraw</h2>
        </div>

        <div className="rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/20 p-6 text-center">
          <div className="w-14 h-14 rounded-full bg-green-500/10 flex items-center justify-center mx-auto mb-4">
            <Check className="w-7 h-7 text-green-400" />
          </div>
          <h3 className="text-lg font-bold text-white mb-2">Withdrawal Submitted</h3>
          <p className="text-xs text-[#8892b0] mb-1">Order No: {orderNo}</p>
          <p className="text-sm text-[#8892b0] mb-5">
            Your withdrawal is being processed. Check transaction history for updates.
          </p>
          <button
            onClick={() => { setOrderNo(''); window.location.href = '/profile'; }}
            className="px-5 py-2.5 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold rounded-lg text-sm active:scale-95 transition-transform"
          >
            View Transaction History
          </button>
          <button
            onClick={() => setOrderNo('')}
            className="mt-3 text-xs text-[#f5a623] font-medium block mx-auto active:opacity-70"
          >
            New Withdrawal
          </button>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <ArrowUpFromLine className="w-5 h-5 text-[#e94560]" />
        <h2 className="text-sm font-bold text-white">Withdraw</h2>
      </div>

      {/* Withdraw Channels */}
      <div className="mb-4">
        <h3 className="text-xs font-semibold text-[#8892b0] mb-2">Withdrawal Method</h3>
        {loading ? (
          <div className="flex items-center justify-center py-6">
            <Loader2 className="w-4 h-4 text-[#f5a623] animate-spin" />
          </div>
        ) : channels.length === 0 ? (
          <p className="text-xs text-[#8892b0] py-4 text-center">No withdrawal channels available</p>
        ) : (
          <div className="grid grid-cols-3 gap-2">
            {channels.map((channel) => {
              const Icon = CHANNEL_ICONS[channel.name?.toLowerCase()] || CreditCard;
              const isSelected = selectedChannel === channel.id;
              return (
                <button
                  key={channel.id}
                  onClick={() => setSelectedChannel(channel.id)}
                  className={`flex flex-col items-center gap-1.5 p-2.5 rounded-xl border transition-all active:scale-95 ${
                    isSelected
                      ? 'bg-[#e94560]/10 border-[#e94560]/40'
                      : 'bg-[#1a1a2e]/60 border-[#f5a623]/10'
                  }`}
                >
                  <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${
                    isSelected ? 'bg-[#e94560]/20' : 'bg-[#1a1a2e]'
                  }`}>
                    <Icon className={`w-4 h-4 ${isSelected ? 'text-[#e94560]' : 'text-[#8892b0]'}`} />
                  </div>
                  <span className={`text-[10px] font-medium ${isSelected ? 'text-[#e94560]' : 'text-[#ccd6f6]'}`}>
                    {channel.name}
                  </span>
                </button>
              );
            })}
          </div>
        )}
      </div>

      {/* Account / Address input */}
      {selectedChannelData?.need_account && (
        <div className="mb-3">
          <h3 className="text-xs font-semibold text-[#8892b0] mb-2">
            {selectedChannelData.name.toLowerCase().includes('bank') ? 'Bank Account' :
             selectedChannelData.name.toLowerCase().includes('upi') ? 'UPI ID' :
             'Wallet Address'}
          </h3>
          <input
            type="text"
            placeholder={
              selectedChannelData.name.toLowerCase().includes('bank') ? 'Enter bank account number' :
              selectedChannelData.name.toLowerCase().includes('upi') ? 'Enter UPI ID (e.g. name@upi)' :
              'Enter wallet address (e.g. TRC20 address)'
            }
            value={account}
            onChange={(e) => { setAccount(e.target.value); setError(''); }}
            className="w-full bg-[#16213e] border border-[#f5a623]/20 rounded-xl px-4 py-3 text-white text-sm outline-none placeholder-[#8892b0]/40 focus:border-[#f5a623]/50 transition-colors"
          />
          {selectedChannelData.name.toLowerCase().includes('bank') && (
            <input
              type="text"
              placeholder="Account holder name"
              value={accountName}
              onChange={(e) => setAccountName(e.target.value)}
              className="w-full bg-[#16213e] border border-[#f5a623]/20 rounded-xl px-4 py-3 text-white text-sm outline-none placeholder-[#8892b0]/40 focus:border-[#f5a623]/50 transition-colors mt-2"
            />
          )}
        </div>
      )}

      {/* Amount input */}
      <div className="mb-3">
        <h3 className="text-xs font-semibold text-[#8892b0] mb-2">Amount</h3>
        <div className="flex items-center bg-[#16213e] border border-[#f5a623]/20 rounded-xl overflow-hidden focus-within:border-[#f5a623]/50 transition-colors">
          <span className="pl-4 text-lg text-[#e94560] font-bold">$</span>
          <input
            type="number"
            inputMode="decimal"
            placeholder="0.00"
            value={amount}
            onChange={(e) => { setAmount(e.target.value); setError(''); }}
            className="flex-1 bg-transparent text-white text-xl font-semibold px-2 py-3 outline-none placeholder-[#8892b0]/40 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
          />
          <button
            onClick={() => {
              const max = selectedChannelData ? Math.min(balance, selectedChannelData.max_amount) : balance;
              setAmount(String(max));
              setError('');
            }}
            className="px-3 py-1 mr-2 rounded-lg bg-[#e94560]/10 text-[#e94560] text-[10px] font-semibold active:bg-[#e94560]/20 whitespace-nowrap"
          >
            MAX
          </button>
        </div>
        {selectedChannelData && (
          <p className="text-[10px] text-[#8892b0] mt-1">
            Min: ${selectedChannelData.min_amount} ~ Max: ${selectedChannelData.max_amount}
            {selectedChannelData.daily_limit > 0 && ` | Daily: $${selectedChannelData.daily_limit.toLocaleString()}`}
          </p>
        )}
      </div>

      {/* Preset amounts */}
      <div className="mb-3">
        <div className="grid grid-cols-4 gap-2">
          {PRESET_AMOUNTS.filter((val) => val <= balance || balance <= 0).map((val) => (
            <button
              key={val}
              onClick={() => handleAmountClick(val)}
              className={`py-2 rounded-xl text-xs font-semibold transition-all active:scale-95 ${
                amount === String(val)
                  ? 'bg-gradient-to-r from-[#e94560] to-[#c0392b] text-white shadow-lg shadow-[#e94560]/20'
                  : 'bg-[#1a1a2e] text-[#ccd6f6] border border-[#f5a623]/10'
              }`}
            >
              ${val}
            </button>
          ))}
        </div>
      </div>

      {/* Notice */}
      <div className="flex items-start gap-2 p-2.5 rounded-xl bg-[#e94560]/5 border border-[#e94560]/10 mb-3">
        <AlertTriangle className="w-3.5 h-3.5 text-[#e94560] flex-shrink-0 mt-0.5" />
        <p className="text-[10px] text-[#8892b0] leading-relaxed">
          Withdrawals are processed within 1-24 hours. Ensure your withdrawal address is correct.
        </p>
      </div>

      {/* Error */}
      {error && (
        <p className="text-xs text-[#e94560] bg-[#e94560]/10 px-3 py-2 rounded-lg mb-3">{error}</p>
      )}

      {/* Submit */}
      <button
        onClick={handleSubmit}
        disabled={submitting || !selectedChannel || !amount || parseFloat(amount) <= 0}
        className="w-full py-3 bg-gradient-to-r from-[#e94560] to-[#c0392b] text-white font-semibold rounded-xl text-sm shadow-lg shadow-[#e94560]/20 disabled:opacity-40 active:scale-[0.98] transition-all"
      >
        {submitting ? (
          <span className="flex items-center justify-center gap-2">
            <Loader2 className="w-4 h-4 animate-spin" />
            Processing...
          </span>
        ) : (
          'Withdraw Now'
        )}
      </button>
    </div>
  );
}

// === Main Wallet Page ===
export default function WalletPage() {
  const { isLoggedIn, assets, fetchAssets } = useAuthStore();
  const searchParams = useSearchParams();
  const initialTab = searchParams.get('tab') === 'withdraw' ? 'withdraw' : 'deposit';
  const [activeTab, setActiveTab] = useState<'deposit' | 'withdraw'>(initialTab);

  // Sync tab when searchParams change (e.g. navigating from /deposit?tab=withdraw)
  useEffect(() => {
    const tab = searchParams.get('tab');
    if (tab === 'withdraw' || tab === 'deposit') {
      setActiveTab(tab);
    }
  }, [searchParams]);

  if (!isLoggedIn) {
    return (
      <div>
        <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />
        <div className="pt-32 px-4 flex flex-col items-center justify-center text-center">
          <Wallet className="w-12 h-12 text-[#8892b0] mb-4" />
          <p className="text-sm text-[#8892b0]">Please log in to access your wallet</p>
        </div>
      </div>
    );
  }

  return (
    <div>
      <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />

      <main className="pt-14 px-4">
        {/* Header */}
        <div className="flex items-center gap-2 mb-4">
          <Wallet className="w-5 h-5 text-[#f5a623]" />
          <h1 className="text-lg font-bold text-white">Wallet</h1>
        </div>

        {/* Balance Card */}
        <div className="p-4 rounded-xl bg-gradient-to-r from-[#f5a623]/10 to-[#e94560]/10 border border-[#f5a623]/20 mb-5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs text-[#8892b0]">Available Balance</p>
              <p className="text-2xl font-bold text-[#f5a623]">
                {(assets?.balance ?? 0).toLocaleString()}
                <span className="text-sm font-normal text-[#8892b0] ml-1">{assets?.currency ?? 'USD'}</span>
              </p>
            </div>
            <button
              onClick={fetchAssets}
              className="w-8 h-8 rounded-full bg-[#1a1a2e] flex items-center justify-center active:bg-[#1a1a2e]/80"
            >
              <CreditCard className="w-4 h-4 text-[#8892b0]" />
            </button>
          </div>
        </div>

        {/* Tab Switcher */}
        <div className="flex gap-1 p-1 bg-[#1a1a2e]/60 rounded-xl border border-[#f5a623]/10 mb-5">
          <button
            onClick={() => setActiveTab('deposit')}
            className={`flex-1 flex items-center justify-center gap-1.5 py-2.5 rounded-lg text-sm font-semibold transition-all ${
              activeTab === 'deposit'
                ? 'bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] shadow-lg shadow-[#f5a623]/20'
                : 'text-[#8892b0] active:text-[#ccd6f6]'
            }`}
          >
            <ArrowDownToLine className="w-4 h-4" />
            Deposit
          </button>
          <button
            onClick={() => setActiveTab('withdraw')}
            className={`flex-1 flex items-center justify-center gap-1.5 py-2.5 rounded-lg text-sm font-semibold transition-all ${
              activeTab === 'withdraw'
                ? 'bg-gradient-to-r from-[#e94560] to-[#c0392b] text-white shadow-lg shadow-[#e94560]/20'
                : 'text-[#8892b0] active:text-[#ccd6f6]'
            }`}
          >
            <ArrowUpFromLine className="w-4 h-4" />
            Withdraw
          </button>
        </div>

        {/* Tab Content */}
        {activeTab === 'deposit' ? (
          <DepositTab onGoBack={() => setActiveTab('deposit')} />
        ) : (
          <WithdrawTab onGoBack={() => setActiveTab('withdraw')} />
        )}
      </main>
    </div>
  );
}
