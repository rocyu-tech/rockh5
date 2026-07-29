'use client';

import { useState, useEffect, useCallback } from 'react';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { shopApi, type Order } from '@/lib/api';
import { getErrorMessage } from "@/lib/api-status";
import { toast } from "sonner";
import { fmtMoney, fmtMoneyPlain } from '@/lib/money';
import {
  TrendingUp,
  ArrowUpRight,
  ArrowDownLeft,
  Clock,
  Loader2,
  ChevronLeft,
  ChevronRight,
  Filter,
  X,
} from 'lucide-react';

interface TransactionHistoryProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type Transaction = Order & {
  currency?: string;
  description?: string;
};

type FilterType = 'all' | 'recharge' | 'withdraw' | 'bonus';

const STATUS_STYLES: Record<string, { bg: string; text: string; label: string }> = {
  pending:   { bg: 'bg-yellow-500/10', text: 'text-yellow-400', label: 'Pending' },
  completed: { bg: 'bg-green-500/10',  text: 'text-green-400',  label: 'Completed' },
  failed:    { bg: 'bg-red-500/10',    text: 'text-red-400',    label: 'Failed' },
  cancelled: { bg: 'bg-gray-500/10',   text: 'text-gray-400',   label: 'Cancelled' },
  processing:{ bg: 'bg-blue-500/10',   text: 'text-blue-400',   label: 'Processing' },
};

const TYPE_ICONS: Record<string, typeof ArrowDownLeft> = {
  recharge: ArrowDownLeft,
  withdraw: ArrowUpRight,
  bonus: TrendingUp,
};

export default function TransactionHistory({ open, onOpenChange }: TransactionHistoryProps) {
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [filter, setFilter] = useState<FilterType>('all');
  const pageSize = 10;

  const fetchTransactions = useCallback(async (p: number, f: FilterType) => {
    setLoading(true);
    try {
      const params: Record<string, unknown> = { page: p, page_size: pageSize };
      if (f !== 'all') params.type = f;
      const res = await shopApi.getOrders(params as { page?: number; page_size?: number });
      const { orders, total } = res.data;
      setTransactions(orders);
      setTotalPages(Math.max(1, Math.ceil(total / pageSize)));
    } catch (err) {
      console.error('[TransactionHistory] fetch error:', err);
      toast.error(getErrorMessage(err));
      setTransactions([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (open) {
      setPage(1);
      fetchTransactions(1, filter);
    }
  }, [open, filter, fetchTransactions]);

  const handlePageChange = (newPage: number) => {
    setPage(newPage);
    fetchTransactions(newPage, filter);
  };

  const handleFilterChange = (f: FilterType) => {
    setFilter(f);
    setPage(1);
  };

  const formatAmount = (tx: Transaction) => {
    const isPositive = tx.type === 'recharge' || tx.type === 'bonus';
    const sign = isPositive ? '+' : '-';
    return `${sign}${fmtMoney(tx.amount, tx.currency || 'USD')}`;
  };

  const formatDate = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleString('en-US', {
        month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit',
      });
    } catch {
      console.warn('date formatting failed');
      return dateStr;
    }
  };

  const filters: { key: FilterType; label: string }[] = [
    { key: 'all', label: 'All' },
    { key: 'recharge', label: 'Deposit' },
    { key: 'withdraw', label: 'Withdraw' },
    { key: 'bonus', label: 'Bonus' },
  ];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="bg-[#0a0a1a] border-[#f5a623]/20 text-[#ccd6f6] rounded-2xl max-w-md max-h-[85vh] overflow-hidden flex flex-col" showCloseButton={false}>
        <DialogTitle className="sr-only">Transaction History</DialogTitle>
        <DialogDescription className="sr-only">View your transaction records</DialogDescription>

        {/* Header */}
        <div className="p-5 pb-3 flex-shrink-0">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-bold text-white flex items-center gap-2">
              <TrendingUp className="w-5 h-5 text-[#f5a623]" />
              Transactions
            </h2>
            <button
              onClick={() => onOpenChange(false)}
              className="w-8 h-8 rounded-full bg-white/10 hover:bg-white/20 flex items-center justify-center transition-colors"
            >
              <X className="w-4 h-4 text-[#ccd6f6]" />
            </button>
          </div>

          {/* Filters */}
          <div className="flex gap-2 overflow-x-auto hide-scrollbar">
            {filters.map((f) => (
              <button
                key={f.key}
                onClick={() => handleFilterChange(f.key)}
                className={`px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap transition-all ${
                  filter === f.key
                    ? 'bg-[#f5a623] text-[#0a0a1a]'
                    : 'bg-[#1a1a2e] text-[#8892b0] hover:text-[#ccd6f6] hover:bg-[#1a1a2e]/80 border border-[#f5a623]/10'
                }`}
              >
                {f.label}
              </button>
            ))}
          </div>
        </div>

        {/* List */}
        <div className="flex-1 overflow-y-auto px-5 min-h-0">
          {loading ? (
            <div className="flex items-center justify-center py-16">
              <Loader2 className="w-6 h-6 text-[#f5a623] animate-spin" />
            </div>
          ) : transactions.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <div className="w-16 h-16 rounded-full bg-[#1a1a2e] flex items-center justify-center mb-4">
                <Clock className="w-7 h-7 text-[#8892b0]" />
              </div>
              <p className="text-sm text-[#8892b0]">No transactions yet</p>
              <p className="text-xs text-[#8892b0]/60 mt-1">Your transaction history will appear here</p>
            </div>
          ) : (
            <div className="space-y-2 pb-4">
              {transactions.map((tx) => {
                const isPositive = tx.type === 'recharge' || tx.type === 'bonus';
                const statusStyle = STATUS_STYLES[tx.status] || STATUS_STYLES.pending;
                const Icon = TYPE_ICONS[tx.type] || TrendingUp;

                return (
                  <div
                    key={tx.id}
                    className="flex items-center gap-3 p-3 rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/5 hover:border-[#f5a623]/15 transition-colors"
                  >
                    {/* Icon */}
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${
                      isPositive ? 'bg-green-500/10' : 'bg-red-500/10'
                    }`}>
                      <Icon className={`w-5 h-5 ${isPositive ? 'text-green-400' : 'text-red-400'}`} />
                    </div>

                    {/* Info */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <p className="text-sm font-medium text-white capitalize">
                          {tx.type}
                        </p>
                        <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${statusStyle.bg} ${statusStyle.text}`}>
                          {statusStyle.label}
                        </span>
                      </div>
                      <p className="text-xs text-[#8892b0] mt-0.5 truncate">
                        {tx.description || tx.order_no}
                      </p>
                    </div>

                    {/* Amount + Date */}
                    <div className="text-right flex-shrink-0">
                      <p className={`text-sm font-semibold ${isPositive ? 'text-green-400' : 'text-red-400'}`}>
                        {formatAmount(tx)}
                      </p>
                      <p className="text-[10px] text-[#8892b0] mt-0.5">
                        {formatDate(tx.created_at)}
                      </p>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between px-5 py-3 border-t border-[#f5a623]/10 flex-shrink-0">
            <button
              onClick={() => handlePageChange(page - 1)}
              disabled={page <= 1}
              className="w-11 h-11 rounded-lg bg-[#1a1a2e] hover:bg-[#1a1a2e]/80 disabled:opacity-30 transition-colors"
            >
              <ChevronLeft className="w-4 h-4 text-[#8892b0]" />
            </button>
            <span className="text-xs text-[#8892b0]">
              Page {page} / {totalPages}
            </span>
            <button
              onClick={() => handlePageChange(page + 1)}
              disabled={page >= totalPages}
              className="w-11 h-11 rounded-lg bg-[#1a1a2e] hover:bg-[#1a1a2e]/80 disabled:opacity-30 transition-colors"
            >
              <ChevronRight className="w-4 h-4 text-[#8892b0]" />
            </button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
