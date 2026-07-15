'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/store/auth';
import { useApiStatusContext } from '@/lib/api-status';
import Navbar from '@/components/Navbar';
import VIPSection from '@/components/VIPSection';
import {
  User,
  Wallet,
  Mail,
  Calendar,
  Crown,
  LogOut,
  TrendingUp,
  Users,
  ChevronRight,
  Shield,
  Settings,
  HelpCircle,
  Package,
  RefreshCw,
  Loader2,
} from 'lucide-react';

export default function ProfilePage() {
  const router = useRouter();
  const { isLoggedIn, user, assets, fetchProfile, fetchAssets, logout } = useAuthStore();
  const [activeTab, setActiveTab] = useState<'main' | 'transactions' | 'vip'>('main');
  const [refreshing, setRefreshing] = useState(false);
  const [transactions, setTransactions] = useState<any[]>([]);
  const [txLoading, setTxLoading] = useState(false);
  const [txFilter, setTxFilter] = useState<string>('all');
  const apiStatus = useApiStatusContext();

  // If not logged in, show login prompt
  useEffect(() => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
    }
  }, [isLoggedIn]);

  const handleRefresh = async () => {
    setRefreshing(true);
    await Promise.all([fetchProfile(), fetchAssets()]);
    setRefreshing(false);
  };

  const fetchTransactions = async (filter: string) => {
    setTxLoading(true);
    try {
      const { shopApi } = await import('@/lib/api');
      const params: Record<string, unknown> = { page: 1, page_size: 20 };
      if (filter !== 'all') params.type = filter;
      const res = await shopApi.getOrders(params as { page?: number; page_size?: number });
      const data = res.data?.data;
      if (Array.isArray(data)) {
        setTransactions(data);
      } else if (data && typeof data === 'object' && 'list' in data) {
        setTransactions((data as any).list);
      }
    } catch {
      setTransactions([]);
    } finally {
      setTxLoading(false);
    }
  };

  useEffect(() => {
    if (activeTab === 'transactions') {
      fetchTransactions(txFilter);
    }
  }, [activeTab, txFilter]);

  const handleLogout = () => {
    logout();
    router.push('/');
  };

  if (!isLoggedIn) {
    return (
      <div>
        <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />
        <div className="pt-32 px-4 flex flex-col items-center justify-center text-center">
          <div className="w-16 h-16 rounded-full bg-[#1a1a2e] flex items-center justify-center mb-4">
            <User className="w-8 h-8 text-[#8892b0]" />
          </div>
          <p className="text-sm text-[#8892b0]">Please log in to view your profile</p>
        </div>
      </div>
    );
  }

  const STATUS_STYLES: Record<string, { bg: string; text: string; label: string }> = {
    pending:    { bg: 'bg-yellow-500/10', text: 'text-yellow-400', label: 'Pending' },
    completed:  { bg: 'bg-green-500/10',  text: 'text-green-400',  label: 'Completed' },
    failed:     { bg: 'bg-red-500/10',    text: 'text-red-400',    label: 'Failed' },
    cancelled:  { bg: 'bg-gray-500/10',   text: 'text-gray-400',   label: 'Cancelled' },
    processing: { bg: 'bg-blue-500/10',   text: 'text-blue-400',   label: 'Processing' },
  };

  return (
    <div>
      <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />

      <main className="pt-14 px-4">
        {/* User profile header */}
        <div className="mt-3 mb-4">
          <div className="flex items-center gap-3">
            <div className="w-16 h-16 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center flex-shrink-0">
              <User className="w-8 h-8 text-white" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-lg font-semibold text-white truncate">
                {user?.nickname || user?.email || 'Player'}
              </p>
              <div className="flex items-center gap-2 mt-0.5">
                <div className="flex items-center gap-1">
                  <Crown className="w-3.5 h-3.5 text-[#f5a623]" />
                  <span className="text-xs text-[#f5a623] font-medium">VIP {user?.vip_level ?? 0}</span>
                </div>
                {user?.email && (
                  <span className="text-xs text-[#8892b0] truncate">{user.email}</span>
                )}
              </div>
            </div>
            <button
              onClick={handleRefresh}
              disabled={refreshing}
              className="w-8 h-8 rounded-full bg-[#1a1a2e] flex items-center justify-center active:bg-[#1a1a2e]/80"
            >
              <RefreshCw className={`w-4 h-4 text-[#8892b0] ${refreshing ? 'animate-spin' : ''}`} />
            </button>
          </div>

          {/* Balance card */}
          <div className="mt-3 p-3 rounded-xl bg-gradient-to-r from-[#f5a623]/10 to-[#e94560]/10 border border-[#f5a623]/20">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Wallet className="w-4 h-4 text-[#f5a623]" />
                <span className="text-xs text-[#8892b0]">Balance</span>
              </div>
              <p className="text-xl font-bold text-[#f5a623]">
                {assets?.balance?.toLocaleString() ?? '0.00'} <span className="text-xs font-normal text-[#8892b0]">{assets?.currency ?? 'USD'}</span>
              </p>
            </div>
          </div>
        </div>

        {/* Tab navigation */}
        <div className="flex gap-1 p-1 bg-[#1a1a2e]/60 rounded-xl mb-4">
          {[
            { id: 'main' as const, label: 'Account' },
            { id: 'transactions' as const, label: 'Transactions' },
            { id: 'vip' as const, label: 'VIP Club' },
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex-1 py-2 rounded-lg text-xs font-medium transition-all ${
                activeTab === tab.id
                  ? 'bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] shadow-lg shadow-[#f5a623]/20'
                  : 'text-[#8892b0] active:text-[#ccd6f6]'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Tab content */}
        {activeTab === 'main' && (
          <div className="space-y-2 mb-6">
            {/* Menu items */}
            {[
              { icon: TrendingUp, label: 'Transaction History', action: () => setActiveTab('transactions'), color: '#4ecdc4' },
              { icon: Package, label: 'Backpack', action: () => router.push('/inventory'), color: '#4ecdc4' },
              { icon: Crown, label: 'VIP Club', action: () => setActiveTab('vip'), color: '#f5a623' },
              { icon: Users, label: 'Agent Program', action: () => {}, desc: 'Earn up to 45% commission', color: '#a855f7' },
            ].map((item) => (
              <button
                key={item.label}
                onClick={item.action}
                className="flex items-center gap-3 w-full p-3.5 rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/10 active:bg-[#1a1a2e] transition-colors text-left"
              >
                <div
                  className="w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0"
                  style={{ backgroundColor: `${item.color}15` }}
                >
                  <item.icon className="w-4.5 h-4.5" style={{ color: item.color }} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-[#ccd6f6]">{item.label}</p>
                  {item.desc && <p className="text-[10px] text-[#8892b0] mt-0.5">{item.desc}</p>}
                </div>
                <ChevronRight className="w-4 h-4 text-[#8892b0]" />
              </button>
            ))}

            {/* Info items */}
            <div className="mt-3 space-y-2">
              {user?.email && (
                <div className="flex items-center gap-3 p-3 rounded-xl bg-[#1a1a2e]/40">
                  <Mail className="w-4 h-4 text-[#8892b0]" />
                  <div className="flex-1 min-w-0">
                    <p className="text-[10px] text-[#8892b0]">Email</p>
                    <p className="text-xs text-[#ccd6f6] truncate">{user.email}</p>
                  </div>
                </div>
              )}
              {user?.created_at && (
                <div className="flex items-center gap-3 p-3 rounded-xl bg-[#1a1a2e]/40">
                  <Calendar className="w-4 h-4 text-[#8892b0]" />
                  <div className="flex-1 min-w-0">
                    <p className="text-[10px] text-[#8892b0]">Joined</p>
                    <p className="text-xs text-[#ccd6f6]">{new Date(user.created_at).toLocaleDateString()}</p>
                  </div>
                </div>
              )}
            </div>

            {/* Logout */}
            <button
              onClick={handleLogout}
              className="flex items-center gap-3 w-full p-3.5 rounded-xl bg-[#e94560]/10 border border-[#e94560]/20 active:bg-[#e94560]/20 transition-colors mt-4"
            >
              <LogOut className="w-4.5 h-4.5 text-[#e94560]" />
              <span className="text-sm font-medium text-[#e94560]">Logout</span>
            </button>
          </div>
        )}

        {activeTab === 'transactions' && (
          <div className="mb-6">
            {/* Filter tabs */}
            <div className="flex gap-2 overflow-x-auto hide-scrollbar mb-3">
              {[
                { key: 'all', label: 'All' },
                { key: 'recharge', label: 'Deposit' },
                { key: 'withdraw', label: 'Withdraw' },
                { key: 'bonus', label: 'Bonus' },
              ].map((f) => (
                <button
                  key={f.key}
                  onClick={() => setTxFilter(f.key)}
                  className={`px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap transition-all ${
                    txFilter === f.key
                      ? 'bg-[#f5a623] text-[#0a0a1a]'
                      : 'bg-[#1a1a2e] text-[#8892b0] border border-[#f5a623]/10'
                  }`}
                >
                  {f.label}
                </button>
              ))}
            </div>

            {/* Transaction list */}
            {txLoading ? (
              <div className="flex items-center justify-center py-16">
                <Loader2 className="w-6 h-6 text-[#f5a623] animate-spin" />
              </div>
            ) : transactions.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-16 text-center">
                <div className="w-14 h-14 rounded-full bg-[#1a1a2e] flex items-center justify-center mb-3">
                  <TrendingUp className="w-6 h-6 text-[#8892b0]" />
                </div>
                <p className="text-xs text-[#8892b0]">No transactions yet</p>
                <p className="text-[10px] text-[#8892b0]/60 mt-1">Your transaction history will appear here</p>
              </div>
            ) : (
              <div className="space-y-2">
                {transactions.map((tx: any) => {
                  const isPositive = tx.type === 'recharge' || tx.type === 'bonus';
                  const statusStyle = STATUS_STYLES[tx.status] || STATUS_STYLES.pending;
                  return (
                    <div
                      key={tx.id}
                      className="flex items-center gap-3 p-3 rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/5"
                    >
                      <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${
                        isPositive ? 'bg-green-500/10' : 'bg-red-500/10'
                      }`}>
                        <TrendingUp className={`w-5 h-5 ${isPositive ? 'text-green-400' : 'text-red-400'}`} />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <p className="text-xs font-medium text-white capitalize">{tx.type}</p>
                          <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${statusStyle.bg} ${statusStyle.text}`}>
                            {statusStyle.label}
                          </span>
                        </div>
                        <p className="text-[10px] text-[#8892b0] mt-0.5 truncate">
                          {tx.description || tx.order_no}
                        </p>
                      </div>
                      <div className="text-right flex-shrink-0">
                        <p className={`text-xs font-semibold ${isPositive ? 'text-green-400' : 'text-red-400'}`}>
                          {isPositive ? '+' : '-'}{tx.amount?.toLocaleString()} {tx.currency || 'USD'}
                        </p>
                        <p className="text-[10px] text-[#8892b0] mt-0.5">
                          {tx.created_at ? new Date(tx.created_at).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : ''}
                        </p>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}

        {activeTab === 'vip' && (
          <div className="mb-6">
            <VIPSection />
          </div>
        )}
      </main>
    </div>
  );
}
