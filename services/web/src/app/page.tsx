"use client";

import React, { useState, useRef } from "react";
import { Header } from "@/components/Header";
import { UploadDropzone } from "@/components/UploadDropzone";
import { ResultsViewer } from "@/components/ResultsViewer";
import { DocumentPreview } from "@/components/DocumentPreview";
import { DocumentDashboard } from "@/components/DocumentDashboard";
import {
  Sparkles,
  ShieldCheck,
  Zap,
  Database,
  Layers,
  BarChart3,
  UploadCloud,
  FileCheck,
} from "lucide-react";

export default function Home() {
  const [activeDocId, setActiveDocId] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<"studio" | "archive">("studio");
  const studioRef = useRef<HTMLDivElement>(null);
  const uploadRef = useRef<HTMLDivElement>(null);

  const handleSelectFromArchive = (id: string) => {
    setActiveDocId(id);
    setActiveTab("studio");
    setTimeout(() => {
      studioRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 100);
  };

  const handleUploadSuccess = (newId: string) => {
    setActiveDocId(newId);
    setActiveTab("studio");
    setTimeout(() => {
      studioRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 100);
  };

  const handleSwitchToUpload = () => {
    setActiveTab("studio");
    setTimeout(() => {
      uploadRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 100);
  };

  return (
    <div className="flex flex-col min-h-screen bg-[#090d16] text-slate-100">
      <Header />

      {/* Hero Section */}
      <section className="relative overflow-hidden pt-12 pb-14 border-b border-slate-900/80">
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[320px] bg-indigo-500/10 blur-[130px] rounded-full pointer-events-none" />

        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 relative">
          <div className="text-center space-y-4 max-w-3xl mx-auto">
            <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-indigo-500/10 border border-indigo-500/20 text-xs font-semibold text-indigo-400 mb-1">
              <Sparkles className="w-3.5 h-3.5" />
              <span>Multimodal AI Document Intelligence</span>
            </div>

            <h1 className="text-4xl sm:text-5xl lg:text-6xl font-extrabold tracking-tight text-white leading-tight">
              Enterprise Document Intelligence,{" "}
              <span className="bg-gradient-to-r from-indigo-400 via-purple-300 to-cyan-400 bg-clip-text text-transparent">
                Simplified.
              </span>
            </h1>

            <p className="text-base sm:text-lg text-slate-400 leading-relaxed">
              Extract, validate, and verify commercial invoices, fiscal receipts, and contracts in English and Amharic with automated regional compliance.
            </p>

            {/* Feature Pills */}
            <div className="pt-2 flex flex-wrap items-center justify-center gap-3 text-xs font-medium text-slate-400">
              <div className="flex items-center gap-1.5 bg-slate-900/80 border border-slate-800 px-3 py-1.5 rounded-xl">
                <Zap className="w-3.5 h-3.5 text-cyan-400" />
                <span>Async FIFO Queue</span>
              </div>
              <div className="flex items-center gap-1.5 bg-slate-900/80 border border-slate-800 px-3 py-1.5 rounded-xl">
                <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
                <span>Human-in-the-Loop Review</span>
              </div>
              <div className="flex items-center gap-1.5 bg-slate-900/80 border border-slate-800 px-3 py-1.5 rounded-xl">
                <FileCheck className="w-3.5 h-3.5 text-amber-400" />
                <span>Ethiopian Fiscal & TIN Validation</span>
              </div>
              <div className="flex items-center gap-1.5 bg-slate-900/80 border border-slate-800 px-3 py-1.5 rounded-xl">
                <Database className="w-3.5 h-3.5 text-purple-400" />
                <span>Postgres & S3 MinIO</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Main Studio / Dashboard Interactive Area */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
        {/* Navigation Tabs */}
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <div className="flex items-center gap-2 bg-slate-900/80 p-1 rounded-2xl border border-slate-800">
            <button
              onClick={() => setActiveTab("studio")}
              className={`inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all ${
                activeTab === "studio"
                  ? "bg-indigo-600 text-white shadow-md shadow-indigo-600/20"
                  : "text-slate-400 hover:text-slate-200 hover:bg-slate-800/50"
              }`}
            >
              <Layers className="w-3.5 h-3.5" />
              <span>Extraction Studio</span>
            </button>

            <button
              onClick={() => setActiveTab("archive")}
              className={`inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all ${
                activeTab === "archive"
                  ? "bg-indigo-600 text-white shadow-md shadow-indigo-600/20"
                  : "text-slate-400 hover:text-slate-200 hover:bg-slate-800/50"
              }`}
            >
              <BarChart3 className="w-3.5 h-3.5" />
              <span>Document Archive & Management</span>
            </button>
          </div>

          {activeTab === "archive" ? (
            <button
              onClick={handleSwitchToUpload}
              className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow-lg shadow-indigo-600/20 transition-all active:scale-95"
            >
              <UploadCloud className="w-3.5 h-3.5" />
              <span>Upload Document</span>
            </button>
          ) : (
            <button
              onClick={() => setActiveTab("archive")}
              className="text-xs text-indigo-400 hover:text-indigo-300 font-medium inline-flex items-center gap-1"
            >
              <span>View all in Archive</span>
              <span>→</span>
            </button>
          )}
        </div>

        {/* Tab 1: Studio Mode */}
        {activeTab === "studio" && (
          <div className="space-y-8">
            {/* Upload Area */}
            <div ref={uploadRef}>
              <UploadDropzone onUploaded={handleUploadSuccess} />
            </div>

            {/* Side-by-Side Studio */}
            <div ref={studioRef} className="space-y-4">
              <div className="flex items-center justify-between border-b border-slate-800/80 pb-3">
                <div className="space-y-0.5">
                  <h2 className="text-base font-bold text-white tracking-tight flex items-center gap-2">
                    <Layers className="w-4 h-4 text-indigo-400" />
                    <span>Side-by-Side Document & Extraction Studio</span>
                  </h2>
                  <p className="text-xs text-slate-400">
                    Inspect original file streaming on the left and review extracted fields with human-in-the-loop approval on the right.
                  </p>
                </div>
              </div>

              {activeDocId ? (
                <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
                  {/* Left Pane: Original Document Preview */}
                  <div className="lg:col-span-6">
                    <DocumentPreview documentId={activeDocId} />
                  </div>

                  {/* Right Pane: Extracted Fields & Metadata */}
                  <div className="lg:col-span-6">
                    <ResultsViewer documentId={activeDocId} />
                  </div>
                </div>
              ) : (
                <div className="bg-slate-950/40 border border-dashed border-slate-800 rounded-3xl p-16 text-center flex flex-col items-center justify-center space-y-4">
                  <div className="h-12 w-12 rounded-2xl bg-slate-900 border border-slate-800 flex items-center justify-center text-slate-500">
                    <Layers className="w-6 h-6" />
                  </div>
                  <div className="space-y-1">
                    <h3 className="text-sm font-semibold text-slate-300">No document selected</h3>
                    <p className="text-xs text-slate-500 max-w-sm">
                      Upload a file above or select an existing record from the Document Archive to start inspecting and verifying.
                    </p>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Tab 2: Document Archive & Dashboard Mode */}
        {activeTab === "archive" && (
          <DocumentDashboard
            onSelectDocument={handleSelectFromArchive}
            selectedId={activeDocId}
            onUploadNew={handleSwitchToUpload}
          />
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-900/80 py-6 text-center text-xs text-slate-500">
        <p>DocIntel • Enterprise Multimodal Document Processing with Go, Python, Redis & Postgres</p>
      </footer>
    </div>
  );
}
