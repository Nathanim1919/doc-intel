"use client";

import React, { useEffect, useState, useMemo } from "react";
import {
  FileText,
  Image as ImageIcon,
  Search,
  RefreshCw,
  CheckCircle2,
  AlertTriangle,
  Clock,
  Zap,
  XCircle,
  Copy,
  Check,
  ExternalLink,
  ArrowUpRight,
  Filter,
  Layers,
  BarChart3,
} from "lucide-react";
import { listDocuments } from "@/lib/api";
import { DocumentItem } from "@/types/api";

interface DocumentDashboardProps {
  onSelectDocument: (id: string) => void;
  selectedId?: string | null;
  onUploadNew?: () => void;
}

type StatusFilter = "ALL" | "REVIEW_REQUIRED" | "COMPLETED" | "PROCESSING" | "FAILED";

export function DocumentDashboard({
  onSelectDocument,
  selectedId,
  onUploadNew,
}: DocumentDashboardProps) {
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [filterStatus, setFilterStatus] = useState<StatusFilter>("ALL");
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const fetchDocuments = async () => {
    try {
      setLoading(true);
      const res = await listDocuments(filterStatus === "ALL" ? undefined : filterStatus);
      setDocuments(res.documents || []);
    } catch (err) {
      console.error("Failed to load documents:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDocuments();
  }, [filterStatus]);

  // Copy UUID helper
  const handleCopyId = (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    navigator.clipboard.writeText(id);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 1800);
  };

  // Format file size
  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
  };

  // Format date
  const formatDate = (isoString: string) => {
    try {
      const d = new Date(isoString);
      return d.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return isoString;
    }
  };

  // Metrics counts calculated across current dataset
  const metrics = useMemo(() => {
    const total = documents.length;
    let reviewCount = 0;
    let completedCount = 0;
    let processingCount = 0;
    let failedCount = 0;

    for (const d of documents) {
      if (d.status === "REVIEW_REQUIRED") reviewCount++;
      else if (d.status === "COMPLETED") completedCount++;
      else if (d.status === "PROCESSING" || d.status === "QUEUED" || d.status === "UPLOADED") processingCount++;
      else if (d.status.includes("FAIL")) failedCount++;
    }

    return { total, reviewCount, completedCount, processingCount, failedCount };
  }, [documents]);

  // Client-side search filtering
  const filteredDocuments = useMemo(() => {
    if (!searchQuery.trim()) return documents;
    const q = searchQuery.toLowerCase();
    return documents.filter(
      (d) =>
        d.filename.toLowerCase().includes(q) ||
        d.id.toLowerCase().includes(q) ||
        d.mime_type.toLowerCase().includes(q)
    );
  }, [documents, searchQuery]);

  const getStatusBadge = (st: string) => {
    switch (st) {
      case "COMPLETED":
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <CheckCircle2 className="w-3.5 h-3.5" />
            COMPLETED
          </span>
        );
      case "REVIEW_REQUIRED":
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <AlertTriangle className="w-3.5 h-3.5" />
            REVIEW REQUIRED
          </span>
        );
      case "PROCESSING":
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 animate-pulse">
            <Zap className="w-3.5 h-3.5" />
            PROCESSING
          </span>
        );
      case "QUEUED":
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-800 text-slate-300 border border-slate-700">
            <Clock className="w-3.5 h-3.5" />
            QUEUED
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-red-500/10 text-red-400 border border-red-500/20">
            <XCircle className="w-3.5 h-3.5" />
            {st}
          </span>
        );
    }
  };

  return (
    <div className="bg-slate-950/70 border border-slate-800 rounded-3xl p-6 sm:p-8 space-y-6 shadow-2xl backdrop-blur-md">
      {/* Header & Action */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-800/80 pb-5">
        <div>
          <div className="flex items-center gap-2">
            <BarChart3 className="w-5 h-5 text-indigo-400" />
            <h3 className="text-lg font-bold text-white tracking-tight">Document Archive & Management</h3>
          </div>
          <p className="text-xs text-slate-400 mt-0.5">
            View all ingested receipts, invoices, and contracts. Click any record to inspect the original preview and review extracted fields.
          </p>
        </div>

        <div className="flex items-center gap-2 self-start sm:self-auto">
          <button
            onClick={fetchDocuments}
            disabled={loading}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-900 hover:bg-slate-800 border border-slate-700 text-xs font-medium text-slate-300 transition-colors"
            title="Refresh documents"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin text-indigo-400" : ""}`} />
            <span>Refresh</span>
          </button>

          {onUploadNew && (
            <button
              onClick={onUploadNew}
              className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow-lg shadow-indigo-600/20 transition-all active:scale-95"
            >
              <span>+ New Upload</span>
            </button>
          )}
        </div>
      </div>

      {/* Metrics Counters */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div
          onClick={() => setFilterStatus("ALL")}
          className={`cursor-pointer rounded-2xl p-4 border transition-all ${
            filterStatus === "ALL"
              ? "bg-slate-900 border-indigo-500/60 shadow-lg shadow-indigo-500/10"
              : "bg-slate-900/40 border-slate-800/80 hover:bg-slate-900/80"
          }`}
        >
          <span className="text-[11px] font-mono text-slate-400 uppercase block mb-1">Total Ingested</span>
          <span className="text-2xl font-bold text-white font-mono">{metrics.total}</span>
        </div>

        <div
          onClick={() => setFilterStatus("REVIEW_REQUIRED")}
          className={`cursor-pointer rounded-2xl p-4 border transition-all ${
            filterStatus === "REVIEW_REQUIRED"
              ? "bg-amber-500/15 border-amber-500/60 shadow-lg shadow-amber-500/10"
              : "bg-slate-900/40 border-slate-800/80 hover:bg-slate-900/80"
          }`}
        >
          <div className="flex items-center justify-between mb-1">
            <span className="text-[11px] font-mono text-amber-400/90 uppercase block">Needs Review</span>
            <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />
          </div>
          <span className="text-2xl font-bold text-amber-300 font-mono">{metrics.reviewCount}</span>
        </div>

        <div
          onClick={() => setFilterStatus("COMPLETED")}
          className={`cursor-pointer rounded-2xl p-4 border transition-all ${
            filterStatus === "COMPLETED"
              ? "bg-emerald-500/15 border-emerald-500/60 shadow-lg shadow-emerald-500/10"
              : "bg-slate-900/40 border-slate-800/80 hover:bg-slate-900/80"
          }`}
        >
          <div className="flex items-center justify-between mb-1">
            <span className="text-[11px] font-mono text-emerald-400/90 uppercase block">Completed</span>
            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
          </div>
          <span className="text-2xl font-bold text-emerald-300 font-mono">{metrics.completedCount}</span>
        </div>

        <div
          onClick={() => setFilterStatus("PROCESSING")}
          className={`cursor-pointer rounded-2xl p-4 border transition-all ${
            filterStatus === "PROCESSING"
              ? "bg-indigo-500/15 border-indigo-500/60 shadow-lg shadow-indigo-500/10"
              : "bg-slate-900/40 border-slate-800/80 hover:bg-slate-900/80"
          }`}
        >
          <div className="flex items-center justify-between mb-1">
            <span className="text-[11px] font-mono text-indigo-400/90 uppercase block">In-Flight</span>
            <Zap className="w-3.5 h-3.5 text-indigo-400" />
          </div>
          <span className="text-2xl font-bold text-indigo-300 font-mono">{metrics.processingCount}</span>
        </div>
      </div>

      {/* Toolbar: Search and Status Tabs */}
      <div className="flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3 pt-2">
        {/* Search */}
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
          <input
            type="text"
            placeholder="Search by filename, document ID, or type..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full bg-slate-900/80 border border-slate-800 rounded-xl pl-9 pr-4 py-2 text-xs text-slate-200 placeholder:text-slate-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
        </div>

        {/* Filter Pills */}
        <div className="flex flex-wrap items-center gap-1.5 bg-slate-900/60 border border-slate-800/80 p-1 rounded-2xl self-start md:self-auto">
          {(["ALL", "REVIEW_REQUIRED", "COMPLETED", "PROCESSING", "FAILED"] as StatusFilter[]).map((st) => (
            <button
              key={st}
              onClick={() => setFilterStatus(st)}
              className={`px-3 py-1 rounded-xl text-xs font-semibold transition-all ${
                filterStatus === st
                  ? "bg-indigo-600 text-white shadow-sm"
                  : "text-slate-400 hover:text-slate-200 hover:bg-slate-800/50"
              }`}
            >
              {st === "ALL" ? "All" : st.replace("_", " ")}
            </button>
          ))}
        </div>
      </div>

      {/* Documents Data Table */}
      <div className="border border-slate-800/80 rounded-2xl overflow-hidden bg-slate-900/20">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs text-slate-300">
            <thead className="bg-slate-900/80 border-b border-slate-800/80 text-[11px] uppercase tracking-wider text-slate-400 font-mono">
              <tr>
                <th className="py-3.5 px-4 font-semibold">Document</th>
                <th className="py-3.5 px-4 font-semibold hidden md:table-cell">ID</th>
                <th className="py-3.5 px-4 font-semibold hidden sm:table-cell">Size</th>
                <th className="py-3.5 px-4 font-semibold hidden lg:table-cell">Uploaded At</th>
                <th className="py-3.5 px-4 font-semibold">Status</th>
                <th className="py-3.5 px-4 font-semibold text-right">Action</th>
              </tr>
            </thead>

            <tbody className="divide-y divide-slate-800/60">
              {loading && documents.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-12 text-center text-slate-500 font-mono">
                    <div className="inline-block h-6 w-6 border-2 border-indigo-500/30 border-t-indigo-500 rounded-full animate-spin mb-2" />
                    <p>Loading documents repository...</p>
                  </td>
                </tr>
              ) : filteredDocuments.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-12 text-center text-slate-500">
                    <div className="max-w-sm mx-auto space-y-2">
                      <p className="font-semibold text-slate-400">No matching documents found</p>
                      <p className="text-[11px] text-slate-600">
                        {searchQuery
                          ? `No records match the search term "${searchQuery}".`
                          : `No documents exist under the filter "${filterStatus}".`}
                      </p>
                      {searchQuery && (
                        <button
                          onClick={() => setSearchQuery("")}
                          className="mt-2 text-xs text-indigo-400 hover:text-indigo-300 font-medium"
                        >
                          Clear search filter
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ) : (
                filteredDocuments.map((doc) => {
                  const isSelected = doc.id === selectedId;
                  const isPDF = doc.mime_type.toLowerCase().includes("pdf");

                  return (
                    <tr
                      key={doc.id}
                      onClick={() => onSelectDocument(doc.id)}
                      className={`cursor-pointer transition-colors group ${
                        isSelected
                          ? "bg-indigo-500/10 border-l-2 border-l-indigo-500"
                          : "hover:bg-slate-900/60"
                      }`}
                    >
                      {/* Document Name */}
                      <td className="py-3.5 px-4">
                        <div className="flex items-center gap-2.5">
                          <div
                            className={`p-2 rounded-xl shrink-0 ${
                              isPDF
                                ? "bg-red-500/10 text-red-400 border border-red-500/20"
                                : "bg-cyan-500/10 text-cyan-400 border border-cyan-500/20"
                            }`}
                          >
                            {isPDF ? <FileText className="w-4 h-4" /> : <ImageIcon className="w-4 h-4" />}
                          </div>
                          <div className="overflow-hidden">
                            <p className="font-medium text-slate-200 truncate max-w-[200px] sm:max-w-xs group-hover:text-indigo-300 transition-colors">
                              {doc.filename}
                            </p>
                            <span className="text-[10px] text-slate-500 uppercase tracking-wider font-mono">
                              {doc.mime_type.split("/")[1]?.toUpperCase() || doc.mime_type}
                            </span>
                          </div>
                        </div>
                      </td>

                      {/* ID with Copy */}
                      <td className="py-3.5 px-4 hidden md:table-cell">
                        <div className="inline-flex items-center gap-1.5 font-mono text-[11px] text-slate-400 bg-slate-900/60 px-2 py-1 rounded-lg border border-slate-800">
                          <span>{doc.id.slice(0, 8)}...</span>
                          <button
                            onClick={(e) => handleCopyId(e, doc.id)}
                            className="text-slate-500 hover:text-slate-300 p-0.5"
                            title="Copy full document ID"
                          >
                            {copiedId === doc.id ? (
                              <Check className="w-3 h-3 text-emerald-400" />
                            ) : (
                              <Copy className="w-3 h-3" />
                            )}
                          </button>
                        </div>
                      </td>

                      {/* Size */}
                      <td className="py-3.5 px-4 hidden sm:table-cell font-mono text-slate-400 text-[11px]">
                        {formatBytes(doc.size_bytes)}
                      </td>

                      {/* Created At */}
                      <td className="py-3.5 px-4 hidden lg:table-cell text-slate-400 text-[11px]">
                        {formatDate(doc.created_at)}
                      </td>

                      {/* Status */}
                      <td className="py-3.5 px-4">
                        {getStatusBadge(doc.status)}
                      </td>

                      {/* Action */}
                      <td className="py-3.5 px-4 text-right">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            onSelectDocument(doc.id);
                          }}
                          className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-medium transition-all ${
                            isSelected
                              ? "bg-indigo-600 text-white shadow-sm"
                              : "bg-slate-800/80 hover:bg-slate-700 text-slate-300"
                          }`}
                        >
                          <span>Inspect</span>
                          <ArrowUpRight className="w-3 h-3" />
                        </button>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
