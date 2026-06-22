'use client';

import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';
import { shopApi } from '@/lib/api';
import Navbar from '@/components/Navbar';
import { ArrowLeft, Building, Smartphone, Zap, CreditCard, Loader2, Check, AlertTriangle, Wallet } from 'lucide-react';

interface WithdrawChannel {
  id: number;
  name: string;
  icon?: string;
  min_amount: number;
  max_amount: number;
  daily_limit: number;
  status: number;
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

export default function WithdrawalPage() {
  const { isLoggedIn, assets, fetchAssets } = useAuthStore();
  const apiStatus = useApiStatusContext();

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
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
      return;
    }
    setLoading(true);
    shopApi.getWithdrawChannels().then((res) => {
      const data = res.data?.data;
      if (Array.isArray(data)) {
        const active = (data as WithdrawChannel[]).filter((c: WithdrawChannel) => c.status === 1);
        setChannels(active);
        if (active.length > 0) setSelectedChannel(active[0].id);
      }
    }).catch(() => {
      // Fallback: show default channels even if API fails
      const fallback: WithdrawChannel[] = [
        { id: 1, name: 'USDT-TRC20', min_amount: 10, max_amount: 50000, daily_limit: 100000, status: 1, need_account: true },
        { id: 2, name: 'Bank Transfer', min_amount: 20, max_amount: 10000, daily_limit: 50000, status: 1, need_account: true },
        { id: 3, name: 'UPI', min_amount: 5, max_amount: 5000, daily_limit: 20000, status: 1, need_account: true },
      ];
      setChannels(fallback);
      setSelectedChannel(fallback[0].id);
    }).finally(() => setLoading(false));
  }, [isLoggedIn]);

  const selectedChannelData = channels.find((c) => c.id === selectedChannel);
  const balance = assets?.balance ?? 0;

  const handleAmountClick = (val: number) => {
    if (val > balance) {
      setError('Insufficient balance');
      return;
    }
    setAmount(String(val));
    setError('');
  };

  const handleAmountChange = (val: string) => {
    setAmount(val);
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

  if (!isLoggedIn) {
    return (
      <div>
        <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />
        <div className="pt-32 px-4 flex flex-col items-center justify-center text-center">
          <Wallet className="w-12 h-12 text-[#8892b0] mb-4" />
          <p className="text-sm text-[#8892b0]">Please log in to withdraw</p>
        </div>
      </div>
    );
  }

  // Success screen
  if (orderNo) {
    return (
      <div>
        <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />
        <div className="pt-14 px-4">
          <button
            onClick={() => setOrderNo('')}
            className="flex items-center gap-2 text-sm text-[#8892b0] mb-4 active:text-[#f5a623]"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to Withdrawal
          </button>

          <div className="rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/20 p-6 text-center">
            <div className="w-14 h-14 rounded-full bg-green-500/10 flex items-center justify-center mx-auto mb-4">
              <Check className="w-7 h-7 text-green-400" />
            </div>
            <h2 className="text-lg font-bold text-white mb-2">Withdrawal Submitted</h2>
            <p className="text-xs text-[#8892b0] mb-1">Order No: {orderNo}</p>
            <p className="text-sm text-[#8892b0] mb-6">
              Your withdrawal is being processed. Please check your transaction history for updates.
            </p>
            <button
              onClick={() => {
                setOrderNo('');
                window.location.href = '/profile';
              }}
              className="px-5 py-2.5 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold rounded-lg text-sm active:scale-95 transition-transform"
            >
              View Transaction History
            </button>
          </div>
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
          <h1 className="text-lg font-bold text-white">Withdrawal</h1>
        </div>

        {/* Balance display */}
        <div className="p-4 rounded-xl bg-gradient-to-r from-[#f5a623]/10 to-[#e94560]/10 border border-[#f5a623]/20 mb-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs text-[#8892b0]">Available Balance</p>
              <p className="text-2xl font-bold text-[#f5a623]">
                {balance.toLocaleString()}
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

        {/* Withdraw Channels */}
        <div className="mb-4">
          <h2 className="text-sm font-semibold text-white mb-2.5">Withdrawal Method</h2>
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="w-5 h-5 text-[#f5a623] animate-spin" />
            </div>
          ) : channels.length === 0 ? (
            <div className="py-8 text-center">
              <p className="text-xs text-[#8892b0]">No withdrawal channels available</p>
            </div>
          ) : (
            <div className="grid grid-cols-3 gap-2">
              {channels.map((channel) => {
                const Icon = CHANNEL_ICONS[channel.name?.toLowerCase()] || CreditCard;
                const isSelected = selectedChannel === channel.id;
                return (
                  <button
                    key={channel.id}
                    onClick={() => setSelectedChannel(channel.id)}
                    className={`flex flex-col items-center gap-2 p-3 rounded-xl border transition-all active:scale-95 ${
                      isSelected
                        ? 'bg-[#f5a623]/10 border-[#f5a623]/40'
                        : 'bg-[#1a1a2e]/60 border-[#f5a623]/10 active:border-[#f5a623]/30'
                    }`}
                  >
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${
                      isSelected ? 'bg-[#f5a623]/20' : 'bg-[#1a1a2e]'
                    }`}>
                      <Icon className={`w-5 h-5 ${isSelected ? 'text-[#f5a623]' : 'text-[#8892b0]'}`} />
                    </div>
                    <span className={`text-[11px] font-medium ${isSelected ? 'text-[#f5a623]' : 'text-[#ccd6f6]'}`}>
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
          <div className="mb-4">
            <h2 className="text-sm font-semibold text-white mb-2.5">
              {selectedChannelData.name.toLowerCase().includes('bank') ? 'Bank Account' :
               selectedChannelData.name.toLowerCase().includes('upi') ? 'UPI ID' :
               'Wallet Address'}
            </h2>
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
        <div className="mb-4">
          <h2 className="text-sm font-semibold text-white mb-2.5">Amount</h2>
          <div className="relative">
            <div className="flex items-center bg-[#16213e] border border-[#f5a623]/20 rounded-xl overflow-hidden focus-within:border-[#f5a623]/50 transition-colors">
              <span className="pl-4 text-lg text-[#f5a623] font-bold">$</span>
              <input
                type="number"
                inputMode="decimal"
                placeholder="0.00"
                value={amount}
                onChange={(e) => handleAmountChange(e.target.value)}
                className="flex-1 bg-transparent text-white text-xl font-semibold px-2 py-3.5 outline-none placeholder-[#8892b0]/40 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
              />
              <button
                onClick={() => {
                  const max = selectedChannelData ? Math.min(balance, selectedChannelData.max_amount) : balance;
                  setAmount(String(max));
                  setError('');
                }}
                className="px-3 py-1.5 mr-2 rounded-lg bg-[#f5a623]/10 text-[#f5a623] text-[10px] font-semibold active:bg-[#f5a623]/20 whitespace-nowrap"
              >
                MAX
              </button>
            </div>
            {selectedChannelData && (
              <p className="text-[10px] text-[#8892b0] mt-1.5">
                Min: ${selectedChannelData.min_amount} ~ Max: ${selectedChannelData.max_amount}
                {selectedChannelData.daily_limit > 0 && ` | Daily Limit: $${selectedChannelData.daily_limit.toLocaleString()}`}
              </p>
            )}
          </div>
        </div>

        {/* Preset amounts */}
        <div className="mb-4">
          <h2 className="text-sm font-semibold text-white mb-2.5">Quick Amount</h2>
          <div className="grid grid-cols-4 gap-2">
            {PRESET_AMOUNTS.filter((val) => val <= balance || balance <= 0).map((val) => (
              <button
                key={val}
                onClick={() => handleAmountClick(val)}
                className={`py-2.5 rounded-xl text-sm font-semibold transition-all active:scale-95 ${
                  amount === String(val)
                    ? 'bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] shadow-lg shadow-[#f5a623]/20'
                    : 'bg-[#1a1a2e] text-[#ccd6f6] border border-[#f5a623]/10 active:border-[#f5a623]/30'
                }`}
              >
                ${val}
              </button>
            ))}
          </div>
        </div>

        {/* Notice */}
        <div className="flex items-start gap-2 p-3 rounded-xl bg-[#e94560]/5 border border-[#e94560]/10 mb-4">
          <AlertTriangle className="w-4 h-4 text-[#e94560] flex-shrink-0 mt-0.5" />
          <div className="text-[10px] text-[#8892b0] leading-relaxed">
            <p className="text-[#e94560] font-medium mb-0.5">Please Note</p>
            <p>Withdrawals are typically processed within 1-24 hours. Please ensure your withdrawal address is correct. Incorrect addresses may result in permanent loss of funds.</p>
          </div>
        </div>

        {/* Error */}
        {error && (
          <p className="text-xs text-[#e94560] bg-[#e94560]/10 px-3 py-2 rounded-lg mb-4">{error}</p>
        )}

        {/* Submit */}
        <button
          onClick={handleSubmit}
          disabled={submitting || !selectedChannel || !amount || parseFloat(amount) <= 0}
          className="w-full py-3.5 bg-gradient-to-r from-[#e94560] to-[#c0392b] text-white font-semibold rounded-xl text-sm shadow-lg shadow-[#e94560]/20 disabled:opacity-40 active:scale-[0.98] transition-all mb-6"
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
      </main>
    </div>
  );
}
