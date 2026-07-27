'use client';

import { useState, useEffect, useCallback } from 'react';
import { Mail, MailOpen, Trash2, Download, Loader2, AlertCircle, ChevronDown, ChevronUp, Bell } from 'lucide-react';
import Navbar from '@/components/Navbar';
import { Button } from '@/components/ui/button';
import { mailApi, MailItem } from '@/lib/api';
import { toast } from 'sonner';

export default function MailPage() {
  const [mails, setMails] = useState<MailItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [claimingId, setClaimingId] = useState<number | null>(null);

  const fetchMails = useCallback(async () => {
    try {
      const res = await mailApi.getMailbox();
      // Proto returns {mails: [...]}, map to flat array for the page
      const data = res.data as Record<string, unknown> | undefined;
      const list = data?.mails || data?.list;
      setMails(Array.isArray(list) ? list : []);
    } catch {
      toast.error('Failed to load mails');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchMails(); }, [fetchMails]);

  const handleRead = async (mail: MailItem) => {
    if (expandedId === mail.mail_id) {
      setExpandedId(null);
      return;
    }
    setExpandedId(mail.mail_id);
    if (mail.read_flag === 0) {
      try {
        await mailApi.readMail(mail.mail_id);
        setMails(prev => prev.map(m =>
          m.mail_id === mail.mail_id ? { ...m, read_flag: 1 } : m
        ));
      } catch { console.warn('[mail] mark read failed'); }
    }
  };

  const handleClaimAttachment = async (mailId: number) => {
    setClaimingId(mailId);
    try {
      await mailApi.claimMailAttachment(mailId);
      toast.success('Attachments claimed!');
      await fetchMails();
    } catch {
      toast.error('Claim failed');
    } finally {
      setClaimingId(null);
    }
  };

  const handleDelete = async (ids: number[]) => {
    try {
      await mailApi.deleteMail(ids);
      toast.success(`Deleted ${ids.length} mail(s)`);
      setSelectedIds(new Set());
      await fetchMails();
    } catch {
      toast.error('Delete failed');
    }
  };

  const toggleSelect = (id: number) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div>
      <Navbar
        onLoginClick={() => window.dispatchEvent(new CustomEvent('auth:logout'))}
        onRegisterClick={() => window.dispatchEvent(new CustomEvent('nav:open-register'))}
      />

      <main className="pt-14 px-4">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Mail className="w-5 h-5 text-[#f5a623]" />
            <h1 className="text-lg font-bold text-white">Mailbox</h1>
          </div>
          {mails.length > 0 && (
            <div className="flex gap-2">
              {selectedIds.size > 0 && (
                <Button
                  size="sm"
                  variant="destructive"
                  onClick={() => handleDelete(Array.from(selectedIds))}
                  className="text-xs h-7"
                >
                  <Trash2 className="w-3 h-3 mr-1" />
                  Delete ({selectedIds.size})
                </Button>
              )}
            </div>
          )}
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-6 h-6 text-[#f5a623] animate-spin" />
          </div>
        ) : mails.length === 0 ? (
          <div className="flex flex-col items-center py-12 text-[#8892b0]">
            <Bell className="w-8 h-8 mb-2" />
            <p className="text-sm">No mail</p>
          </div>
        ) : (
          <div className="space-y-2">
            {mails.map(mail => {
              const isExpanded = expandedId === mail.mail_id;
              const hasAttachment = mail.attachment && mail.attachment.length > 0;
              const isSelected = selectedIds.has(mail.mail_id);

              return (
                <div
                  key={mail.mail_id}
                  className={`bg-[#0d1117] rounded-xl border transition-all ${
                    isSelected ? 'border-[#f5a623]' : 'border-[#1e293b]'
                  } ${mail.read_flag === 0 ? 'border-l-2 border-l-[#f5a623]' : ''}`}
                >
                  <div
                    className="flex items-center gap-3 p-3 cursor-pointer"
                    onClick={() => handleRead(mail)}
                  >
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={(e) => {
                        e.stopPropagation();
                        toggleSelect(mail.mail_id);
                      }}
                      className="w-3.5 h-3.5 accent-[#f5a623] flex-shrink-0"
                    />
                    <div className="flex-shrink-0">
                      {mail.read_flag === 1 ? (
                        <MailOpen className="w-4 h-4 text-[#8892b0]" />
                      ) : (
                        <Mail className="w-4 h-4 text-[#f5a623]" />
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <h3 className={`text-xs font-medium truncate ${mail.read_flag === 0 ? 'text-white' : 'text-[#8892b0]'}`}>
                          {mail.title}
                        </h3>
                        {hasAttachment && (
                          <span className="text-[9px] bg-[#f5a623]/20 text-[#f5a623] px-1.5 py-0.5 rounded-full flex-shrink-0">
                            {mail.attachment!.length} item(s)
                          </span>
                        )}
                      </div>
                      <p className="text-[10px] text-[#8892b0] mt-0.5">
                        {mail.from_name} · {new Date(mail.created_at).toLocaleDateString()}
                      </p>
                    </div>
                    {isExpanded ? (
                      <ChevronUp className="w-4 h-4 text-[#8892b0] flex-shrink-0" />
                    ) : (
                      <ChevronDown className="w-4 h-4 text-[#8892b0] flex-shrink-0" />
                    )}
                  </div>

                  {isExpanded && (
                    <div className="px-3 pb-3 border-t border-[#1e293b]">
                      <div className="mt-2 text-xs text-[#8892b0] whitespace-pre-wrap">{mail.content}</div>

                      {hasAttachment && (
                        <div className="mt-3 space-y-1.5">
                          <p className="text-[10px] text-[#8892b0] font-medium">Attachments:</p>
                          {mail.attachment!.map((att, i) => (
                            <div key={i} className="flex items-center gap-2 bg-[#1e293b] rounded-lg px-3 py-1.5">
                              {att.icon && <img src={att.icon} alt="" className="w-4 h-4" />}
                              <span className="text-xs text-white flex-1">{att.item_name}</span>
                              <span className="text-xs text-[#f5a623]">x{att.quantity}</span>
                            </div>
                          ))}
                          {mail.receive_flag === 0 && (
                            <Button
                              size="sm"
                              onClick={() => handleClaimAttachment(mail.mail_id)}
                              disabled={claimingId === mail.mail_id}
                              className="w-full mt-2 bg-[#f5a623] text-black text-xs h-7 hover:opacity-90"
                            >
                              {claimingId === mail.mail_id ? (
                                <Loader2 className="w-3 h-3 animate-spin" />
                              ) : (
                                <>
                                  <Download className="w-3 h-3 mr-1" />
                                  Claim Attachments
                                </>
                              )}
                            </Button>
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </main>
    </div>
  );
}
