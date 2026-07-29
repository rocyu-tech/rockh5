'use client';

import { useState, useEffect, useCallback } from 'react';
import { Users, Copy, TrendingUp, DollarSign, UserPlus, ChevronRight, Loader2, AlertCircle, ExternalLink, BarChart3, Link2, Wallet } from 'lucide-react';
import Navbar from '@/components/Navbar';
import { useAppStore } from '@/store/app';
import { Button } from '@/components/ui/button';
import { agentApi, AgentInfo, SubordinateItem, CommissionSummary, CommissionRecord } from '@/lib/api';
import { toast } from 'sonner';
import { getErrorMessage } from "@/lib/api-status";

export default function AgentPage() {
  const [agentInfo, setAgentInfo] = useState<AgentInfo | null>(null);
  const [commissionSummary, setCommissionSummary] = useState<CommissionSummary | null>(null);
  const [subordinates, setSubordinates] = useState<SubordinateItem[]>([]);
  const [records, setRecords] = useState<CommissionRecord[]>([]);
  const [activeTab, setActiveTab] = useState<'overview' | 'subordinates' | 'records'>('overview');
  const [loading, setLoading] = useState(true);
  const [generatingLink, setGeneratingLink] = useState(false);
  const [settling, setSettling] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      const [infoRes, summaryRes, subsRes, recordsRes] = await Promise.all([
        agentApi.getAgentInfo(),
        agentApi.getDashboard(),
        agentApi.getSubordinates(1, 20),
        agentApi.getCommissionRecords(1, 20),
      ]);
      setAgentInfo(infoRes.data);
      setCommissionSummary(summaryRes.data);
      setSubordinates(subsRes.data?.list || []);
      setRecords(recordsRes.data?.list || []);
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleApply = async () => {
    toast.info('Please contact support to become an agent');
  };

  const handleGenerateLink = async () => {
    setGeneratingLink(true);
    try {
      const res = await agentApi.getPromoLink();
      setAgentInfo(prev => prev ? { ...prev, referral_link: res.data.referral_link, referral_code: res.data.referral_code } : prev);
      toast.success('Promo link generated!');
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setGeneratingLink(false);
    }
  };

  const handleCopyReferral = () => {
    if (agentInfo?.referral_link) {
      navigator.clipboard.writeText(agentInfo.referral_link);
      toast.success('Referral link copied!');
    }
  };

  const handleSettlement = async () => {
    setSettling(true);
    try {
      await agentApi.requestSettlement();
      toast.success('Settlement requested!');
      await fetchData();
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setSettling(false);
    }
  };

  if (loading) {
    return (
      <div>
        <Navbar
        onLoginClick={() => useAppStore.getState().requestLogin()}
        onRegisterClick={() => useAppStore.getState().requestRegister()}
        />
        <div className="flex items-center justify-center pt-24">
          <Loader2 className="w-6 h-6 text-[#f5a623] animate-spin" />
        </div>
      </div>
    );
  }

  // Not an agent yet
  if (agentInfo && agentInfo.agent_status === 0) {
    return (
      <div>
        <Navbar
        onLoginClick={() => useAppStore.getState().requestLogin()}
        onRegisterClick={() => useAppStore.getState().requestRegister()}
        />
        <main className="pt-14 px-4">
          <div className="flex items-center gap-2 mb-4">
            <Users className="w-5 h-5 text-[#f5a623]" />
            <h1 className="text-lg font-bold text-white">Agent Center</h1>
          </div>
          <div className="bg-[#0d1117] rounded-xl border border-[#1e293b] p-6 text-center">
            <div className="w-16 h-16 mx-auto mb-4 bg-[#f5a623]/10 rounded-full flex items-center justify-center">
              <UserPlus className="w-8 h-8 text-[#f5a623]" />
            </div>
            <h2 className="text-lg font-bold text-white mb-2">Become an Agent</h2>
            <p className="text-xs text-[#8892b0] mb-6 max-w-xs mx-auto">
              Earn commissions by referring new players. Get up to 40% commission on your referrals&apos; bets!
            </p>
            <Button
              onClick={handleApply}
              className="bg-gradient-to-r from-[#f5a623] to-[#e09100] text-black font-semibold hover:opacity-90"
            >
              <UserPlus className="w-4 h-4 mr-2" />
              Apply Now
            </Button>
          </div>
        </main>
      </div>
    );
  }

  return (
    <div>
      <Navbar
        onLoginClick={() => useAppStore.getState().requestLogin()}
        onRegisterClick={() => useAppStore.getState().requestRegister()}
      />

      <main className="pt-14 px-4">
        <div className="flex items-center gap-2 mb-4">
          <Users className="w-5 h-5 text-[#f5a623]" />
          <h1 className="text-lg font-bold text-white">Agent Center</h1>
        </div>

        {/* Referral Link */}
        <div className="bg-[#0d1117] rounded-xl border border-[#1e293b] p-3 mb-4">
          <div className="flex items-center gap-2 mb-2">
            <Link2 className="w-4 h-4 text-[#8892b0]" />
            <p className="text-[10px] text-[#8892b0]">Your Referral Link</p>
          </div>
          <div className="flex items-center gap-2">
            <div className="flex-1 min-w-0 bg-[#1e293b] rounded-lg px-3 py-2">
              <p className="text-xs text-white truncate">{agentInfo?.referral_link || 'No link generated yet'}</p>
            </div>
            <Button
              size="sm"
              onClick={handleCopyReferral}
              disabled={!agentInfo?.referral_link}
              className="bg-[#f5a623] text-black text-xs h-8 px-3 hover:opacity-90 flex-shrink-0"
            >
              <Copy className="w-3 h-3" />
            </Button>
            <Button
              size="sm"
              onClick={handleGenerateLink}
              disabled={generatingLink}
              variant="outline"
              className="text-xs h-8 px-3 border-[#f5a623] text-[#f5a623] hover:bg-[#f5a623]/10 flex-shrink-0"
            >
              {generatingLink ? <Loader2 className="w-3 h-3 animate-spin" /> : 'Generate'}
            </Button>
          </div>
        </div>

        {/* Commission Summary Cards + Settlement */}
        {commissionSummary && (
          <>
            <div className="grid grid-cols-2 gap-3 mb-3">
              {[
                { label: 'Today', value: commissionSummary.today_commission, icon: TrendingUp },
                { label: 'Available', value: commissionSummary.available_commission, icon: DollarSign },
                { label: 'This Month', value: commissionSummary.this_month_commission, icon: BarChart3 },
                { label: 'Total Earned', value: commissionSummary.total_commission, icon: DollarSign },
              ].map(card => {
                const Icon = card.icon;
                return (
                  <div key={card.label} className="bg-[#0d1117] rounded-xl border border-[#1e293b] p-3">
                    <div className="flex items-center gap-1.5 mb-1">
                      <Icon className="w-3 h-3 text-[#8892b0]" />
                      <span className="text-[10px] text-[#8892b0]">{card.label}</span>
                    </div>
                    <p className="text-sm font-bold text-[#f5a623]">{card.value.toLocaleString()}</p>
                  </div>
                );
              })}
            </div>

            {/* Settlement Button */}
            {commissionSummary.available_commission > 0 && (
              <Button
                onClick={handleSettlement}
                disabled={settling}
                className="w-full mb-4 bg-gradient-to-r from-[#4ecdc4] to-[#2db5a8] text-black font-semibold hover:opacity-90 h-10"
              >
                {settling ? (
                  <Loader2 className="w-4 h-4 animate-spin mr-2" />
                ) : (
                  <Wallet className="w-4 h-4 mr-2" />
                )}
                Request Settlement ({commissionSummary.available_commission.toLocaleString()})
              </Button>
            )}
          </>
        )}

        {/* Tabs */}
        <div className="flex gap-2 mb-4">
          {[
            { key: 'overview', label: 'Overview' },
            { key: 'subordinates', label: `Subordinates (${subordinates.length})` },
            { key: 'records', label: 'Records' },
          ].map(tab => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key as typeof activeTab)}
              className={`px-3 py-1.5 rounded-full text-xs font-medium transition-all ${
                activeTab === tab.key
                  ? 'bg-[#f5a623] text-black'
                  : 'bg-[#1e293b] text-[#8892b0] hover:bg-[#2d3a5c]'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Overview Tab */}
        {activeTab === 'overview' && agentInfo && (
          <div className="space-y-3">
            <div className="bg-[#0d1117] rounded-xl border border-[#1e293b] p-4">
              <h3 className="text-xs font-medium text-white mb-3">Agent Info</h3>
              <div className="space-y-2">
                {[
                  { label: 'Agent Level', value: `Level ${agentInfo.agent_level}` },
                  { label: 'Commission Rate', value: `${(agentInfo.commission_rate * 100).toFixed(1)}%` },
                  { label: 'Total Subordinates', value: agentInfo.subordinate_count },
                  { label: 'Direct Subordinates', value: agentInfo.direct_subordinate_count },
                  { label: 'Referral Code', value: agentInfo.referral_code },
                ].map(item => (
                  <div key={item.label} className="flex justify-between items-center">
                    <span className="text-[11px] text-[#8892b0]">{item.label}</span>
                    <span className="text-xs text-white font-medium">{item.value}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* Subordinates Tab */}
        {activeTab === 'subordinates' && (
          <div className="space-y-2">
            {subordinates.length === 0 ? (
              <div className="flex flex-col items-center py-12 text-[#8892b0]">
                <AlertCircle className="w-8 h-8 mb-2" />
                <p className="text-sm">No subordinates yet</p>
              </div>
            ) : (
              subordinates.map(sub => (
                <div key={sub.user_id} className="bg-[#0d1117] rounded-xl border border-[#1e293b] p-3 flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-[#1e293b] overflow-hidden flex-shrink-0">
                    {sub.avatar ? (
                      <img src={sub.avatar} alt="" className="w-full h-full object-cover" />
                    ) : (
                      <div className="w-full h-full flex items-center justify-center text-sm">👤</div>
                    )}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-white font-medium truncate">{sub.nickname}</p>
                    <p className="text-[10px] text-[#8892b0]">VIP {sub.vip_level} · Bet: {sub.total_bet.toLocaleString()}</p>
                  </div>
                  <span className="text-xs text-[#f5a623] flex-shrink-0">+{sub.commission.toLocaleString()}</span>
                </div>
              ))
            )}
          </div>
        )}

        {/* Records Tab */}
        {activeTab === 'records' && (
          <div className="space-y-2">
            {records.length === 0 ? (
              <div className="flex flex-col items-center py-12 text-[#8892b0]">
                <AlertCircle className="w-8 h-8 mb-2" />
                <p className="text-sm">No commission records</p>
              </div>
            ) : (
              records.map(rec => (
                <div key={rec.record_id} className="bg-[#0d1117] rounded-xl border border-[#1e293b] p-3 flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-[#f5a623]/10 flex items-center justify-center flex-shrink-0">
                    <DollarSign className="w-4 h-4 text-[#f5a623]" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-white font-medium truncate">
                      {rec.commission_type === 1 ? 'Bet Commission' : rec.commission_type === 2 ? 'Deposit Commission' : 'Commission'}
                    </p>
                    <p className="text-[10px] text-[#8892b0]">
                      {rec.nickname} · {new Date(rec.created_at).toLocaleDateString()}
                    </p>
                  </div>
                  <span className="text-xs text-[#f5a623] font-medium flex-shrink-0">+{rec.amount.toLocaleString()}</span>
                </div>
              ))
            )}
          </div>
        )}
      </main>
    </div>
  );
}
