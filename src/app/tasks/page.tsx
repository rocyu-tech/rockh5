'use client';

import { useState, useEffect, useCallback } from 'react';
import { CheckCircle2, Gift, Trophy, Zap, Clock, Star, Target, ChevronRight, Loader2, AlertCircle } from 'lucide-react';
import Navbar from '@/components/Navbar';
import { Button } from '@/components/ui/button';
import { taskApi, TaskTypeState, TaskItem } from '@/lib/api';
import { toast } from 'sonner';

const TASK_TYPE_TABS = [
  { key: 0, label: 'Daily Tasks', icon: Clock },
  { key: 1, label: 'Weekly Tasks', icon: Trophy },
  { key: 2, label: 'Growth Tasks', icon: Target },
];

export default function TasksPage() {
  const [activeTab, setActiveTab] = useState(0);
  const [taskStates, setTaskStates] = useState<TaskTypeState[]>([]);
  const [loading, setLoading] = useState(true);
  const [claimingId, setClaimingId] = useState<number | null>(null);

  const fetchTasks = useCallback(async () => {
    try {
      const res = await taskApi.getTaskConfig();
      if (res.data?.code === 0) {
        setTaskStates(res.data.data);
      }
    } catch {
      toast.error('Failed to load tasks');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchTasks(); }, [fetchTasks]);

  const handleClaim = async (taskId: number) => {
    setClaimingId(taskId);
    try {
      const res = await taskApi.claimReward(taskId);
      if (res.data?.code === 0) {
        toast.success(`Received: ${res.data.data.item_name} x${res.data.data.quantity}`);
        await fetchTasks();
      } else {
        toast.error(res.data?.msg || 'Claim failed');
      }
    } catch {
      toast.error('Claim failed');
    } finally {
      setClaimingId(null);
    }
  };

  const handleClaimAll = async (taskType: number) => {
    try {
      const res = await taskApi.claimAllRewards(taskType);
      if (res.data?.code === 0) {
        const count = res.data.data.count;
        toast.success(`Claimed ${count} rewards!`);
        await fetchTasks();
      } else {
        toast.error(res.data?.msg || 'Claim all failed');
      }
    } catch {
      toast.error('Claim all failed');
    }
  };

  const currentTasks = taskStates.find(t => t.task_type === activeTab)?.task_type_state || [];
  const canClaimAll = currentTasks.some(t => t.receive_status === 1 && t.task_status === 2);

  const renderProgressBar = (task: TaskItem) => {
    const pct = Math.min(100, Math.round((task.task_progress / task.task_target) * 100));
    return (
      <div className="w-full mt-2">
        <div className="flex justify-between text-[10px] text-[#8892b0] mb-1">
          <span>{task.task_progress}/{task.task_target}</span>
          <span>{pct}%</span>
        </div>
        <div className="h-1.5 bg-[#1e293b] rounded-full overflow-hidden">
          <div
            className="h-full bg-gradient-to-r from-[#f5a623] to-[#e09100] rounded-full transition-all"
            style={{ width: `${pct}%` }}
          />
        </div>
      </div>
    );
  };

  return (
    <div>
      <Navbar
        onLoginClick={() => window.dispatchEvent(new CustomEvent('auth:logout'))}
        onRegisterClick={() => window.dispatchEvent(new CustomEvent('nav:open-register'))}
      />

      <main className="pt-14 px-4">
        <div className="flex items-center gap-2 mb-4">
          <CheckCircle2 className="w-5 h-5 text-[#f5a623]" />
          <h1 className="text-lg font-bold text-white">Tasks</h1>
        </div>

        {/* Tabs */}
        <div className="flex gap-2 mb-4 overflow-x-auto no-scrollbar">
          {TASK_TYPE_TABS.map(tab => {
            const Icon = tab.icon;
            const count = taskStates.find(t => t.task_type === tab.key)?.task_type_state
              ?.filter(t => t.receive_status === 1).length || 0;
            return (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap transition-all ${
                  activeTab === tab.key
                    ? 'bg-[#f5a623] text-black'
                    : 'bg-[#1e293b] text-[#8892b0] hover:bg-[#2d3a5c]'
                }`}
              >
                <Icon className="w-3.5 h-3.5" />
                {tab.label}
                {count > 0 && (
                  <span className="ml-1 bg-red-500 text-white text-[9px] px-1.5 rounded-full">{count}</span>
                )}
              </button>
            );
          })}
        </div>

        {/* Claim All Button */}
        {canClaimAll && (
          <Button
            onClick={() => handleClaimAll(activeTab)}
            className="w-full mb-4 bg-gradient-to-r from-[#f5a623] to-[#e09100] text-black font-semibold hover:opacity-90"
          >
            <Gift className="w-4 h-4 mr-2" />
            Claim All Rewards
          </Button>
        )}

        {/* Task List */}
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-6 h-6 text-[#f5a623] animate-spin" />
          </div>
        ) : currentTasks.length === 0 ? (
          <div className="flex flex-col items-center py-12 text-[#8892b0]">
            <AlertCircle className="w-8 h-8 mb-2" />
            <p className="text-sm">No tasks available</p>
          </div>
        ) : (
          <div className="space-y-3">
            {currentTasks.map(task => (
              <div
                key={task.task_id}
                className="bg-[#0d1117] rounded-xl border border-[#1e293b] p-4"
              >
                <div className="flex items-start gap-3">
                  <div className="w-10 h-10 rounded-lg bg-[#f5a623]/10 flex items-center justify-center flex-shrink-0">
                    {task.task_icon ? (
                      <img src={task.task_icon} alt="" className="w-6 h-6" />
                    ) : (
                      <Zap className="w-5 h-5 text-[#f5a623]" />
                    )}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-medium text-white truncate">{task.task_name}</h3>
                      <span className="text-xs font-bold text-[#f5a623] ml-2 whitespace-nowrap">
                        +{task.task_reward}
                      </span>
                    </div>
                    <p className="text-[11px] text-[#8892b0] mt-0.5 line-clamp-1">{task.task_description}</p>
                    {renderProgressBar(task)}
                  </div>
                  <div className="flex-shrink-0 ml-2">
                    {task.receive_status === 0 && task.task_status === 2 ? (
                      <Button
                        size="sm"
                        onClick={() => handleClaim(task.task_id)}
                        disabled={claimingId === task.task_id}
                        className="bg-[#f5a623] text-black text-xs h-7 px-3 hover:opacity-90"
                      >
                        {claimingId === task.task_id ? (
                          <Loader2 className="w-3 h-3 animate-spin" />
                        ) : (
                          'Claim'
                        )}
                      </Button>
                    ) : task.task_status === 1 ? (
                      task.link_url ? (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => window.open(task.link_url, '_blank')}
                          className="text-xs h-7 px-3 border-[#f5a623] text-[#f5a623] hover:bg-[#f5a623]/10"
                        >
                          Go
                          <ChevronRight className="w-3 h-3 ml-0.5" />
                        </Button>
                      ) : (
                        <span className="text-[10px] text-[#8892b0]">In Progress</span>
                      )
                    ) : (
                      <CheckCircle2 className="w-5 h-5 text-green-500" />
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </main>
    </div>
  );
}
