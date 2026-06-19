'use client';

import { Database } from 'lucide-react';

interface DemoBadgeProps {
  show: boolean;
  label?: string;
}

export default function DemoBadge({ show, label = 'Demo Data' }: DemoBadgeProps) {
  if (!show) return null;

  return (
    <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-[#f5a623]/10 border border-[#f5a623]/20 self-start">
      <Database className="w-3 h-3 text-[#f5a623]" />
      <span className="text-[10px] font-medium text-[#f5a623]">{label}</span>
    </div>
  );
}
