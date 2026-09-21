"use client";

import React, { useEffect, useState } from "react";
import { FileText, RefreshCw, ChevronRight } from "lucide-react";
import { listDocuments } from "@/lib/api";
import { DocumentItem } from "@/types/api";

interface RecentDocumentsProps {
  onSelect: (id: string) => void;
  selectedId?: string | null;
}

export function RecentDocuments({ onSelect, selectedId }: RecentDocumentsProps) {
  const [docs, setDocs] = useState<DocumentItem[]>([]);
  const [loading, setLoading] = useState(false);

  const fetchDocs = async () => {
    try {
      setLoading(true);
      const res = await listDocuments();
      setDocs(res.documents || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDocs();
  }, []);

  return (
    <div className="bg-slate-950/70 border border-slate-800 rounded-3xl p-6 shadow-2xl space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-slate-200">Recent Ingestions</h3>
        <button
          onClick={fetchDocs}
          className="p-1.5 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
          title="Refresh queue"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin text-indigo-400" : ""}`} />
        </button>
      </div>

      {docs.length === 0 ? (
        <p className="text-xs text-slate-500 py-4 text-center">No documents uploaded yet.</p>
      ) : (
        <div className="space-y-2 max-h-72 overflow-y-auto pr-1">
          {docs.map((d) => {
            const isSelected = d.id === selectedId;
            return (
              <div
                key={d.id}
                onClick={() => onSelect(d.id)}
                className={`p-3 rounded-2xl border cursor-pointer transition-all flex items-center justify-between ${
                  isSelected
                    ? "border-indigo-500 bg-indigo-500/10 shadow-lg shadow-indigo-500/10"
                    : "border-slate-800/80 hover:border-slate-700 bg-slate-900/40 hover:bg-slate-900/80"
                }`}
              >
                <div className="flex items-center gap-3 overflow-hidden">
                  <div className="p-2 rounded-xl bg-slate-800/60 text-indigo-400 shrink-0">
                    <FileText className="w-4 h-4" />
                  </div>
                  <div className="overflow-hidden">
                    <p className="text-xs font-semibold text-slate-200 truncate">{d.filename}</p>
                    <p className="text-[11px] font-mono text-slate-500 truncate">{d.id.slice(0, 8)}...</p>
                  </div>
                </div>

                <div className="flex items-center gap-2 shrink-0">
                  <span
                    className={`text-[10px] font-semibold px-2 py-0.5 rounded-full ${
                      d.status === "COMPLETED"
                        ? "bg-emerald-500/10 text-emerald-400"
                        : d.status === "REVIEW_REQUIRED"
                        ? "bg-amber-500/10 text-amber-400"
                        : "bg-slate-800 text-slate-400"
                    }`}
                  >
                    {d.status}
                  </span>
                  <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
