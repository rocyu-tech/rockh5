'use client';

import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { useAuthStore } from '@/store/auth';
import { accountApi } from '@/lib/api';
import { User, Wallet, Mail, Calendar, Crown, Loader2, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { fmtMoney, fmtMoneyPlain } from '@/lib/money';

interface ProfileModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export default function ProfileModal({ open, onOpenChange }: ProfileModalProps) {
  const { user, assets, fetchProfile, fetchAssets } = useAuthStore();
  const [refreshing, setRefreshing] = useState(false);

  useEffect(() => {
    if (open) {
      fetchProfile();
      fetchAssets();
    }
  }, [open, fetchProfile, fetchAssets]);

  const handleRefresh = async () => {
    setRefreshing(true);
    await Promise.all([fetchProfile(), fetchAssets()]);
    setRefreshing(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="bg-[#0a0a1a] border-[#f5a623]/20 text-[#ccd6f6] rounded-2xl max-w-sm" showCloseButton={false}>
        <DialogTitle className="sr-only">User Profile</DialogTitle>
        <DialogDescription className="sr-only">View your account information</DialogDescription>

        <div className="p-6">
          {/* Header */}
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-xl font-bold text-white flex items-center gap-2">
              <User className="w-5 h-5 text-[#f5a623]" />
              Profile
            </h2>
            <Button
              variant="ghost"
              size="icon"
              onClick={handleRefresh}
              disabled={refreshing}
              className="text-[#8892b0] hover:text-[#f5a623] hover:bg-[#f5a623]/10"
            >
              <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
            </Button>
          </div>

          {/* Avatar */}
          <div className="flex flex-col items-center mb-6">
            <div className="w-20 h-20 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center mb-3">
              <User className="w-10 h-10 text-white" />
            </div>
            <p className="text-lg font-semibold text-white">
              {user?.nickname || user?.email || 'Player'}
            </p>
            <div className="flex items-center gap-1 mt-1">
              <Crown className="w-3.5 h-3.5 text-[#f5a623]" />
              <span className="text-sm text-[#f5a623]">VIP {user?.vip_level ?? 0}</span>
            </div>
          </div>

          {/* Info cards */}
          <div className="space-y-3">
            <div className="flex items-center gap-3 p-3 rounded-lg bg-[#1a1a2e] border border-[#f5a623]/10">
              <Mail className="w-4 h-4 text-[#8892b0]" />
              <div className="flex-1 min-w-0">
                <p className="text-xs text-[#8892b0]">Email</p>
                <p className="text-sm text-white truncate">{user?.email || '-'}</p>
              </div>
            </div>

            {user?.phone && (
              <div className="flex items-center gap-3 p-3 rounded-lg bg-[#1a1a2e] border border-[#f5a623]/10">
                <User className="w-4 h-4 text-[#8892b0]" />
                <div className="flex-1 min-w-0">
                  <p className="text-xs text-[#8892b0]">Phone</p>
                  <p className="text-sm text-white truncate">{user.phone}</p>
                </div>
              </div>
            )}

            {user?.created_at && (
              <div className="flex items-center gap-3 p-3 rounded-lg bg-[#1a1a2e] border border-[#f5a623]/10">
                <Calendar className="w-4 h-4 text-[#8892b0]" />
                <div className="flex-1 min-w-0">
                  <p className="text-xs text-[#8892b0]">Joined</p>
                  <p className="text-sm text-white truncate">{new Date(user.created_at).toLocaleDateString()}</p>
                </div>
              </div>
            )}

            <div className="flex items-center gap-3 p-3 rounded-lg bg-gradient-to-r from-[#f5a623]/10 to-[#e94560]/10 border border-[#f5a623]/20">
              <Wallet className="w-4 h-4 text-[#f5a623]" />
              <div className="flex-1">
                <p className="text-xs text-[#8892b0]">Balance</p>
                <p className="text-lg font-bold text-[#f5a623]">
                  {fmtMoney(assets?.balance ?? 0, assets?.currency ?? 'USD')}
                </p>
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
