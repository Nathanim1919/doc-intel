"use client";

import React, { useEffect, useState } from "react";
import { CheckCircle2, AlertTriangle, XCircle, Clock, Zap, Cpu, Hash, FileCheck } from "lucide-react";
import { getDocumentStatus, getDocumentResults } from "@/lib/api";
import { ExtractionResults } from "@/types/api";

interface ResultsViewerProps {
  documentId: string;
}

export function ResultsViewer({ documentId }: ResultsViewerProps) {
  const [status, setStatus] = useState<string>("QUEUED");
  const [results, setResults] = useState<ExtractionResults | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Poll for status until completed / review_required / failed
  useEffect(() => {
    let interval: NodeJS.Timeout;
    let isMounted = true;

    async function check() {
      try {
        const s = await getDocumentStatus(documentId);
        if (!isMounted) return;
        setStatus(s.status);

        if (s.status === "COMPLETED" || s.status === "REVIEW_REQUIRED") {
          clearInterval(interval);
          const r = await getDocumentResults(documentId);
          if (isMounted) {
            setResults(r);
            setLoading(false);
          }
        } else if (s.status.includes("FAIL")) {
          clearInterval(interval);
          setLoading(false);
          setError(`Processing ended with status: ${s.status}`);
        }
      } catch (err: unknown) {
        if (!isMounted) return;
        // 404 on results while still processing is normal
      }
    }

    check();
    interval = setInterval(check, 1500);

    return () => {
      isMounted = false;
      clearInterval(interval);
    };
  }, [documentId]);

  const getStatusBadge = (st: string) => {
    switch (st) {
      case "COMPLETED":
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <CheckCircle2 className="w-3.5 h-3.5" />
            COMPLETED
          </span>
        );
      case "REVIEW_REQUIRED":
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <AlertTriangle className="w-3.5 h-3.5" />
            REVIEW REQUIRED
          </span>
        );
      case "PROCESSING":
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 animate-pulse">
            <Zap className="w-3.5 h-3.5" />
            PROCESSING (GEMINI)
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-slate-800 text-slate-300 border border-slate-700">
            <Clock className="w-3.5 h-3.5" />
            {st}
          </span>
        );
    }
  };

  return (
    <div className="bg-slate-950/70 border border-slate-800 rounded-3xl p-6 sm:p-8 space-y-6 shadow-2xl backdrop-blur-md">
      {/* Header Info */}
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800/80 pb-5">
        <div>
          <span className="text-[11px] font-mono text-slate-500 uppercase tracking-wider block">Document ID</span>
          <span className="text-sm font-mono text-slate-300 select-all">{documentId}</span>
        </div>
        <div>
          {getStatusBadge(status)}
        </div>
      </div>

      {loading && !results && (
        <div className="py-12 flex flex-col items-center justify-center space-y-3">
          <div className="h-10 w-10 border-2 border-indigo-500/30 border-t-indigo-500 rounded-full animate-spin" />
          <p className="text-xs text-slate-400 font-mono">
            Waiting for worker to pop job & run multimodal extraction...
          </p>
        </div>
      )}

      {error && (
        <div className="p-4 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-sm flex items-center gap-2">
          <XCircle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {results && (
        <div className="space-y-6">
          {/* Metadata chips */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div className="bg-slate-900/60 border border-slate-800/80 rounded-2xl p-3.5">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <Cpu className="w-3.5 h-3.5 text-indigo-400" />
                <span>Model</span>
              </div>
              <p className="text-xs font-semibold text-slate-200 font-mono truncate">{results.run.model}</p>
            </div>

            <div className="bg-slate-900/60 border border-slate-800/80 rounded-2xl p-3.5">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <Clock className="w-3.5 h-3.5 text-cyan-400" />
                <span>Latency</span>
              </div>
              <p className="text-xs font-semibold text-slate-200 font-mono">{results.run.latency_ms ?? 0} ms</p>
            </div>

            <div className="bg-slate-900/60 border border-slate-800/80 rounded-2xl p-3.5">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <Hash className="w-3.5 h-3.5 text-purple-400" />
                <span>Tokens</span>
              </div>
              <p className="text-xs font-semibold text-slate-200 font-mono">{results.run.tokens_used ?? 0}</p>
            </div>

            <div className="bg-slate-900/60 border border-slate-800/80 rounded-2xl p-3.5">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <FileCheck className="w-3.5 h-3.5 text-emerald-400" />
                <span>Fields</span>
              </div>
              <p className="text-xs font-semibold text-slate-200 font-mono">{results.fields.length}</p>
            </div>
          </div>

          {/* Fields Table */}
          <div className="space-y-3">
            <h4 className="text-xs font-bold uppercase tracking-wider text-slate-400">Extracted Fields</h4>
            {results.fields.length === 0 ? (
              <div className="p-6 rounded-2xl border border-dashed border-slate-800 text-center text-xs text-slate-400">
                No key-value fields detected by Gemini on this document page.
              </div>
            ) : (
              <div className="border border-slate-800/80 rounded-2xl overflow-hidden divide-y divide-slate-800/60">
                {results.fields.map((field) => {
                  const conf = field.computed_confidence ?? field.raw_confidence ?? 0;
                  const isHigh = conf >= 0.85;
                  const isMid = conf >= 0.50 && conf < 0.85;

                  return (
                    <div key={field.id} className="p-4 bg-slate-900/30 hover:bg-slate-900/60 transition-colors flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                      <div className="space-y-0.5">
                        <span className="text-xs font-mono font-semibold text-indigo-300">
                          {field.field_name}
                        </span>
                        <p className="text-sm font-medium text-slate-100">
                          {field.value || <span className="text-slate-500 italic">null</span>}
                        </p>
                      </div>

                      <div className="flex items-center gap-3 self-end sm:self-auto">
                        <span
                          className={`text-[11px] font-mono font-semibold px-2.5 py-0.5 rounded-full border ${
                            isHigh
                              ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                              : isMid
                              ? "bg-amber-500/10 text-amber-400 border-amber-500/20"
                              : "bg-red-500/10 text-red-400 border-red-500/20"
                          }`}
                        >
                          {(conf * 100).toFixed(0)}% conf
                        </span>

                        <span
                          className={`text-[10px] uppercase font-bold tracking-wider px-2 py-0.5 rounded-md ${
                            field.validation_status === "PASSED"
                              ? "bg-slate-800 text-slate-300"
                              : field.validation_status === "FAILED"
                              ? "bg-red-500/20 text-red-300 border border-red-500/30"
                              : "bg-slate-800/40 text-slate-500"
                          }`}
                        >
                          {field.validation_status}
                        </span>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
