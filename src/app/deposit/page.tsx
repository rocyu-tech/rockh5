'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/store/auth';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';
import { shopApi } from '@/lib/api';
import Navbar from '@/components/Navbar';
import { Wallet, ArrowLeft, CreditCard, Building, Smartphone, Zap, Check, Loader2, Copy, ExternalLink } from 'lucide-react';

interface PaymentChannel {
  id: number;
  name: string;
  icon?: string;
  min_amount: number;
  max_amount: number;
  status: number;
}

const PRESET_AMOUNTS = [10, 20, 50, 100, 200, 500, 1000, 2000];

const CHANNEL_ICONS: Record<string, typeof CreditCard> = {
  usdt: CreditCard,
  crypto: Zap,
  bank: Building,
  upi: Smartphone,
};

export default function DepositPage() {
  const router = useRouter();
  const { isLoggedIn, assets, fetchAssets } = useAuthStore();
  const apiStatus = useApiStatusContext();

  const [channels, setChannels] = useState<PaymentChannel[]>([]);
  const [selectedChannel, setSelectedChannel] = useState<number | null>(null);
  const [amount, setAmount] = useState('');
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [payUrl, setPayUrl] = useState('');
  const [orderNo, setOrderNo] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
      return;
    }
    setLoading(true);
    shopApi.getPaymentChannels().then((res) => {
      const data = res.data?.data;
      if (Array.isArray(data)) {
        const active = (data as PaymentChannel[]).filter((c: PaymentChannel) => c.status === 1);
        setChannels(active);
        if (active.length > 0) setSelectedChannel(active[0].id);
      }
    }).catch((err) => {
      apiStatus.markFailed('shop/payment-channels', getErrorMessage(err));
    }).finally(() => setLoading(false));
  }, [isLoggedIn]);

  const selectedChannelData = channels.find((c) => c.id === selectedChannel);

  const handleAmountClick = (val: number) => {
    setAmount(String(val));
  };

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
      setError(`Minimum amount is ${selectedChannelData.min_amount}`);
      return;
    }
    if (selectedChannelData && numAmount > selectedChannelData.max_amount) {
      setError(`Maximum amount is ${selectedChannelData.max_amount}`);
      return;
    }

    setError('');
    setSubmitting(true);
    try {
      const res = await shopApi.recharge({
        channel_id: selectedChannel,
        amount: numAmount,
      });
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

  const handleCopyAddress = (text: string) => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(text);
    }
  };

  if (!isLoggedIn) {
    return (
      <div>
        <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />
        <div className="pt-32 px-4 flex flex-col items-center justify-center text-center">
          <Wallet className="w-12 h-12 text-[#8892b0] mb-4" />
          <p className="text-sm text-[#8892b0]">Please log in to deposit</p>
        </div>
      </div>
    );
  }

  // Payment redirect screen
  if (payUrl) {
    return (
      <div>
        <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />
        <div className="pt-14 px-4">
          <button
            onClick={() => {
              setPayUrl('');
              setOrderNo('');
              fetchAssets();
            }}
            className="flex items-center gap-2 text-sm text-[#8892b0] mb-4 active:text-[#f5a623]"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to Deposit
          </button>

          <div className="rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/20 p-6 text-center">
            <div className="w-14 h-14 rounded-full bg-green-500/10 flex items-center justify-center mx-auto mb-4">
              <Check className="w-7 h-7 text-green-400" />
            </div>
            <h2 className="text-lg font-bold text-white mb-2">Order Created</h2>
            <p className="text-xs text-[#8892b0] mb-1">Order No: {orderNo}</p>
            <p className="text-sm text-[#8892b0] mb-4">
              Redirecting to payment page...
            </p>
            <div className="flex gap-3 justify-center">
              <button
                onClick={() => window.open(payUrl, '_blank')}
                className="px-5 py-2.5 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold rounded-lg text-sm flex items-center gap-2 active:scale-95 transition-transform"
              >
                <ExternalLink className="w-4 h-4" />
                Open Payment
              </button>
            </div>
            {payUrl && (
              <button
                onClick={() => handleCopyAddress(payUrl)}
                className="mt-3 text-xs text-[#8892b0] flex items-center gap-1 mx-auto active:text-[#f5a623]"
              >
                <Copy className="w-3.5 h-3.5" />
                Copy Payment Link
              </button>
            )}
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
          <h1 className="text-lg font-bold text-white">Deposit</h1>
        </div>

        {/* Balance display */}
        <div className="p-4 rounded-xl bg-gradient-to-r from-[#f5a623]/10 to-[#e94560]/10 border border-[#f5a623]/20 mb-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs text-[#8892b0]">Current Balance</p>
              <p className="text-2xl font-bold text-[#f5a623]">
                {assets?.balance?.toLocaleString() ?? '0.00'}
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

        {/* Payment Channels */}
        <div className="mb-4">
          <h2 className="text-sm font-semibold text-white mb-2.5">Payment Method</h2>
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="w-5 h-5 text-[#f5a623] animate-spin" />
            </div>
          ) : channels.length === 0 ? (
            <div className="py-8 text-center">
              <p className="text-xs text-[#8892b0]">No payment channels available</p>
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
                onChange={(e) => setAmount(e.target.value)}
                className="flex-1 bg-transparent text-white text-xl font-semibold px-2 py-3.5 outline-none placeholder-[#8892b0]/40 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
              />
            </div>
            {selectedChannelData && (
              <p className="text-[10px] text-[#8892b0] mt-1.5">
                Min: ${selectedChannelData.min_amount} ~ Max: ${selectedChannelData.max_amount}
              </p>
            )}
          </div>
        </div>

        {/* Preset amounts */}
        <div className="mb-4">
          <h2 className="text-sm font-semibold text-white mb-2.5">Quick Amount</h2>
          <div className="grid grid-cols-4 gap-2">
            {PRESET_AMOUNTS.map((val) => (
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

        {/* Error */}
        {error && (
          <p className="text-xs text-[#e94560] bg-[#e94560]/10 px-3 py-2 rounded-lg mb-4">{error}</p>
        )}

        {/* Submit */}
        <button
          onClick={handleSubmit}
          disabled={submitting || !selectedChannel || !amount}
          className="w-full py-3.5 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold rounded-xl text-sm shadow-lg shadow-[#f5a623]/20 disabled:opacity-40 active:scale-[0.98] transition-all mb-6"
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
      </main>
    </div>
  );
}
