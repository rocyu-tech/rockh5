'use client';

import { useState, useEffect, useRef } from 'react';
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
  Camera,
  Lock,
  X,
  History,
  KeyRound,
  Languages,
} from 'lucide-react';
import { accountApi } from '@/lib/api';
import { toast } from 'sonner';
import { useLocale } from '@/i18n/provider';

interface Transaction {
  id: number;
  type: 'recharge' | 'withdraw' | 'bonus' | string;
  status: 'pending' | 'completed' | 'failed' | 'cancelled' | 'processing' | string;
  amount: number;
  currency?: string;
  description?: string;
  order_no?: string;
  created_at?: string;
}

export default function ProfilePage() {
  const router = useRouter();
  const { isLoggedIn, user, assets, fetchProfile, fetchAssets, logout, unreadMailCount } = useAuthStore();
  const { locale, setLocale } = useLocale();
  const currentLocaleName = locale === 'zh' ? '中文' : 'English';
  const [activeTab, setActiveTab] = useState<'main' | 'transactions' | 'vip'>('main');
  const [refreshing, setRefreshing] = useState(false);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [txLoading, setTxLoading] = useState(false);
  const [txFilter, setTxFilter] = useState<string>('all');
  const apiStatus = useApiStatusContext();
  const [showChangePassword, setShowChangePassword] = useState(false);
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [showEditProfile, setShowEditProfile] = useState(false);
  const [editNickname, setEditNickname] = useState('');
  const [editPhone, setEditPhone] = useState('');
  const [savingProfile, setSavingProfile] = useState(false);
  const [showDeleteAccount, setShowDeleteAccount] = useState(false);
  // P1: language switcher state
  const [showLangSwitcher, setShowLangSwitcher] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // If not logged in, show login prompt
  useEffect(() => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
    }
  }, [isLoggedIn]);

  // unreadMailCount is managed by auth store (polled in AppProvider)

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
        setTransactions(data as Transaction[]);
      } else if (data && typeof data === 'object' && 'list' in data) {
        setTransactions((data as { list: Transaction[] }).list);
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

  const handleAvatarUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > 5 * 1024 * 1024) {
      toast.error('Avatar must be under 5MB');
      return;
    }
    setUploadingAvatar(true);
    try {
      const res = await accountApi.uploadAvatar(file);
      if (res.data?.code === 0) {
        toast.success('Avatar updated!');
        await fetchProfile();
      } else {
        toast.error(res.data?.message || 'Upload failed');
      }
    } catch {
      toast.error('Upload failed');
    } finally {
      setUploadingAvatar(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleChangePassword = async () => {
    if (!oldPassword || !newPassword) {
      toast.error('Please fill in all fields');
      return;
    }
    if (newPassword !== confirmPassword) {
      toast.error('New passwords do not match');
      return;
    }
    if (newPassword.length < 6) {
      toast.error('Password must be at least 6 characters');
      return;
    }
    setChangingPassword(true);
    try {
      const res = await accountApi.changePassword({
        old_password: oldPassword,
        new_password: newPassword,
      });
      if (res.data?.code === 0) {
        toast.success('Password changed successfully!');
        setShowChangePassword(false);
        setOldPassword('');
        setNewPassword('');
        setConfirmPassword('');
      } else {
        toast.error(res.data?.message || 'Change failed');
      }
    } catch {
      toast.error('Change failed');
    } finally {
      setChangingPassword(false);
    }
  };

  const handleSaveProfile = async () => {
    setSavingProfile(true);
    try {
      const res = await accountApi.updateProfile({
        nickname: editNickname || undefined,
        phone: editPhone || undefined,
      });
      if (res.data?.code === 0) {
        toast.success('Profile updated!');
        setShowEditProfile(false);
        await fetchProfile();
      } else {
        toast.error(res.data?.message || 'Update failed');
      }
    } catch {
      toast.error('Update failed');
    } finally {
      setSavingProfile(false);
    }
  };

  const handleDeleteAccount = async () => {
    setDeleting(true);
    try {
      const res = await accountApi.deleteAccount();
      if (res.data?.code === 0) {
        toast.success('Account deleted');
        logout();
        router.push('/');
      } else {
        toast.error(res.data?.message || 'Delete failed');
      }
    } catch {
      toast.error('Delete failed');
    } finally {
      setDeleting(false);
      setShowDeleteAccount(false);
    }
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
            <div
              className="w-16 h-16 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center flex-shrink-0 relative overflow-hidden cursor-pointer group"
              onClick={() => fileInputRef.current?.click()}
            >
              {user?.avatar ? (
                <img src={user.avatar} alt="" className="w-full h-full object-cover" />
              ) : (
                <User className="w-8 h-8 text-white" />
              )}
              <div className="absolute inset-0 bg-black/50 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity">
                {uploadingAvatar ? (
                  <Loader2 className="w-5 h-5 text-white animate-spin" />
                ) : (
                  <Camera className="w-5 h-5 text-white" />
                )}
              </div>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={handleAvatarUpload}
              />
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
              { icon: Mail, label: 'Mailbox', action: () => router.push('/mail'), color: '#60a5fa', badge: unreadMailCount },
              { icon: Crown, label: 'VIP Club', action: () => router.push('/vip'), color: '#f5a623' },
              { icon: History, label: 'Game History', action: () => router.push('/history'), color: '#a855f7' },
              { icon: Lock, label: 'Change Password', action: () => setShowChangePassword(true), color: '#60a5fa' },
              { icon: Settings, label: 'Edit Profile', action: () => setShowEditProfile(true), color: '#8892b0' },
              { icon: Users, label: 'Agent Program', action: () => router.push('/agent'), desc: 'Earn up to 45% commission', color: '#a855f7' },
              { icon: KeyRound, label: 'Forgot Password', action: () => router.push('/forgot-password'), color: '#8892b0' },
              { icon: Languages, label: 'Language', action: () => setShowLangSwitcher(true), desc: currentLocaleName, color: '#60a5fa' },
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
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium text-[#ccd6f6]">{item.label}</p>
                    {'badge' in item && item.badge ? (
                      <span className="min-w-[18px] h-[18px] flex items-center justify-center bg-red-500 text-white text-[10px] font-bold rounded-full px-1">
                        {item.badge > 99 ? '99+' : item.badge}
                      </span>
                    ) : null}
                  </div>
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

      {/* Change Password Modal */}
      {showChangePassword && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div className="bg-[#0d1117] rounded-2xl border border-[#1e293b] w-full max-w-sm p-5">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-bold text-white">Change Password</h2>
              <button onClick={() => setShowChangePassword(false)} className="text-[#8892b0] hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-3">
              <div>
                <label className="text-[11px] text-[#8892b0] mb-1 block">Current Password</label>
                <input
                  type="password"
                  value={oldPassword}
                  onChange={e => setOldPassword(e.target.value)}
                  className="w-full px-3 py-2 bg-[#1e293b] rounded-lg text-sm text-white border border-[#2d3a5c] focus:border-[#f5a623] focus:outline-none"
                  placeholder="Enter current password"
                />
              </div>
              <div>
                <label className="text-[11px] text-[#8892b0] mb-1 block">New Password</label>
                <input
                  type="password"
                  value={newPassword}
                  onChange={e => setNewPassword(e.target.value)}
                  className="w-full px-3 py-2 bg-[#1e293b] rounded-lg text-sm text-white border border-[#2d3a5c] focus:border-[#f5a623] focus:outline-none"
                  placeholder="Enter new password"
                />
              </div>
              <div>
                <label className="text-[11px] text-[#8892b0] mb-1 block">Confirm New Password</label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={e => setConfirmPassword(e.target.value)}
                  className="w-full px-3 py-2 bg-[#1e293b] rounded-lg text-sm text-white border border-[#2d3a5c] focus:border-[#f5a623] focus:outline-none"
                  placeholder="Confirm new password"
                />
              </div>
            </div>
            <div className="flex gap-2 mt-5">
              <button
                onClick={() => setShowChangePassword(false)}
                className="flex-1 py-2.5 rounded-lg bg-[#1e293b] text-[#8892b0] text-sm font-medium hover:bg-[#2d3a5c] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleChangePassword}
                disabled={changingPassword}
                className="flex-1 py-2.5 rounded-lg bg-[#f5a623] text-black text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                {changingPassword ? <Loader2 className="w-4 h-4 animate-spin mx-auto" /> : 'Confirm'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Edit Profile Modal */}
      {showEditProfile && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div className="bg-[#0d1117] rounded-2xl border border-[#1e293b] w-full max-w-sm p-5">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-bold text-white">Edit Profile</h2>
              <button onClick={() => setShowEditProfile(false)} className="text-[#8892b0] hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-3">
              <div>
                <label className="text-[11px] text-[#8892b0] mb-1 block">Nickname</label>
                <input
                  type="text"
                  value={editNickname}
                  onChange={e => setEditNickname(e.target.value)}
                  className="w-full px-3 py-2 bg-[#1e293b] rounded-lg text-sm text-white border border-[#2d3a5c] focus:border-[#f5a623] focus:outline-none"
                  placeholder={user?.nickname || 'Enter nickname'}
                />
              </div>
              <div>
                <label className="text-[11px] text-[#8892b0] mb-1 block">Phone</label>
                <input
                  type="tel"
                  value={editPhone}
                  onChange={e => setEditPhone(e.target.value)}
                  className="w-full px-3 py-2 bg-[#1e293b] rounded-lg text-sm text-white border border-[#2d3a5c] focus:border-[#f5a623] focus:outline-none"
                  placeholder={user?.phone || 'Enter phone number'}
                />
              </div>
            </div>
            <div className="flex gap-2 mt-5">
              <button
                onClick={() => setShowEditProfile(false)}
                className="flex-1 py-2.5 rounded-lg bg-[#1e293b] text-[#8892b0] text-sm font-medium hover:bg-[#2d3a5c] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleSaveProfile}
                disabled={savingProfile}
                className="flex-1 py-2.5 rounded-lg bg-[#f5a623] text-black text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                {savingProfile ? <Loader2 className="w-4 h-4 animate-spin mx-auto" /> : 'Save'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Account Confirmation */}
      {showDeleteAccount && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div className="bg-[#0d1117] rounded-2xl border border-[#e94560]/30 w-full max-w-sm p-5">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-bold text-[#e94560]">Delete Account</h2>
              <button onClick={() => setShowDeleteAccount(false)} className="text-[#8892b0] hover:text-white">
                <X className="w-5 h-5" />
              </button>
            </div>
            <p className="text-xs text-[#8892b0] mb-5">
              This action is irreversible. All your data, balance, and progress will be permanently deleted.
            </p>
            <div className="flex gap-2">
              <button
                onClick={() => setShowDeleteAccount(false)}
                className="flex-1 py-2.5 rounded-lg bg-[#1e293b] text-[#8892b0] text-sm font-medium hover:bg-[#2d3a5c] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleDeleteAccount}
                disabled={deleting}
                className="flex-1 py-2.5 rounded-lg bg-[#e94560] text-white text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                {deleting ? <Loader2 className="w-4 h-4 animate-spin mx-auto" /> : 'Delete Forever'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* P1: Language Switcher Modal */}
      {showLangSwitcher && (
        <div
          className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={() => setShowLangSwitcher(false)}
        >
          <div
            className="w-full sm:max-w-sm bg-[#1a1a2e] border border-[#f5a623]/20 rounded-t-2xl sm:rounded-2xl p-4 space-y-2"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-base font-bold text-white">
                <Languages className="w-4 h-4 inline mr-2 text-[#f5a623]" />
                Language
              </h3>
              <button
                onClick={() => setShowLangSwitcher(false)}
                className="text-[#8892b0] hover:text-white"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            {([
              { code: 'en' as const, label: 'English', native: 'English' },
              { code: 'zh' as const, label: 'Chinese', native: '中文' },
            ]).map((opt) => (
              <button
                key={opt.code}
                onClick={() => {
                  setLocale(opt.code);
                  setShowLangSwitcher(false);
                  toast.success(opt.native);
                }}
                className={`w-full p-3 rounded-lg flex items-center justify-between transition-colors ${
                  locale === opt.code
                    ? 'bg-[#f5a623]/15 border border-[#f5a623]/40'
                    : 'bg-[#0a0a1a] hover:bg-[#16213e] border border-transparent'
                }`}
              >
                <span className="text-sm font-medium text-white">{opt.native}</span>
                {locale === opt.code && (
                  <span className="text-[10px] px-2 py-0.5 rounded-full bg-[#f5a623] text-black font-bold">
                    ACTIVE
                  </span>
                )}
              </button>
            ))}
            <p className="text-[10px] text-[#8892b0] text-center pt-2">
              More languages coming soon (vi, th)
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
