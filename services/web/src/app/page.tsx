"use client";

import React, { useState } from "react";
import { Header } from "@/components/Header";
import { UploadDropzone } from "@/components/UploadDropzone";
import { ResultsViewer } from "@/components/ResultsViewer";
import { RecentDocuments } from "@/components/RecentDocuments";
import { Sparkles, ArrowRight, ShieldCheck, Zap, Database, Layers } from "lucide-react";

export default function Home() {
  const [activeDocId, setActiveDocId] = useState<string | null>(null);

  return (
    <div className="flex flex-col min-h-screen bg-[#090d16] text-slate-100">
      <Header />

      {/* Hero Section with SEO Value */}
      <section className="relative overflow-hidden pt-12 pb-16 border-b border-slate-900/80">
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[350px] bg-indigo-500/10 blur-[130px] rounded-full pointer-events-none" />

        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 relative">
          <div className="text-center space-y-4 max-w-3xl mx-auto">
            <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-indigo-500/10 border border-indigo-500/20 text-xs font-semibold text-indigo-400 mb-2">
              <Sparkles className="w-3.5 h-3.5" />
              <span>Next-Gen Gemini Flash Multimodal Engine</span>
            </div>

            <h1 className="text-4xl sm:text-5xl lg:text-6xl font-extrabold tracking-tight text-white leading-tight">
              Enterprise Document Intelligence,{" "}
              <span className="bg-gradient-to-r from-indigo-400 via-purple-300 to-cyan-400 bg-clip-text text-transparent">
                Simplified.
              </span>
            </h1>

            <p className="text-base sm:text-lg text-slate-400 leading-relaxed">
              Upload invoices, passports, contracts, or tax forms. DocIntel runs asynchronous FIFO extraction with calibrated confidence scoring, sub-second latency, and automatic review routing.
            </p>

            {/* Feature Pills */}
            <div className="pt-2 flex flex-wrap items-center justify-center gap-4 text-xs font-medium text-slate-400">
              <div className="flex items-center gap-1.5 bg-slate-900/80 border border-slate-800 px-3 py-1.5 rounded-xl">
                <Zap className="w-3.5 h-3.5 text-cyan-400" />
                <span>Async Redis Pipeline</span>
              </div>
              <div className="flex items-center gap-1.5 bg-slate-900/80 border border-slate-800 px-3 py-1.5 rounded-xl">
                <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
                <span>Calibrated Confidence</span>
              </div>
              <div className="flex items-center gap-1.5 bg-slate-900/80 border border-slate-800 px-3 py-1.5 rounded-xl">
                <Database className="w-3.5 h-3.5 text-purple-400" />
                <span>Postgres & MinIO Storage</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Main Studio / Live Interactive Area */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-10">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
          {/* Left Column: Upload & Recent Queue */}
          <div className="lg:col-span-5 space-y-6">
            <div className="space-y-2">
              <h2 className="text-base font-bold text-white tracking-tight flex items-center gap-2">
                <span>Upload Document</span>
              </h2>
              <p className="text-xs text-slate-400">
                Drop any PDF or image to trigger the asynchronous Gemini extraction worker.
              </p>
            </div>

            <UploadDropzone
              onUploaded={(newId) => {
                setActiveDocId(newId);
              }}
            />

            <RecentDocuments
              onSelect={(id) => setActiveDocId(id)}
              selectedId={activeDocId}
            />
          </div>

          {/* Right Column: Live Extraction Inspector */}
          <div className="lg:col-span-7 space-y-6">
            <div className="space-y-2">
              <h2 className="text-base font-bold text-white tracking-tight flex items-center gap-2">
                <Layers className="w-4 h-4 text-indigo-400" />
                <span>Live Extraction Inspector</span>
              </h2>
              <p className="text-xs text-slate-400">
                Real-time status tracking, model metadata, and structured key-value confidence breakdown.
              </p>
            </div>

            {activeDocId ? (
              <ResultsViewer documentId={activeDocId} />
            ) : (
              <div className="bg-slate-950/40 border border-dashed border-slate-800 rounded-3xl p-12 text-center flex flex-col items-center justify-center space-y-4">
                <div className="h-12 w-12 rounded-2xl bg-slate-900 border border-slate-800 flex items-center justify-center text-slate-500">
                  <Layers className="w-6 h-6" />
                </div>
                <div className="space-y-1">
                  <h3 className="text-sm font-semibold text-slate-300">No document selected</h3>
                  <p className="text-xs text-slate-500 max-w-sm">
                    Upload a new document on the left or select an existing one from recent ingestions to view extraction results.
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-900/80 py-6 text-center text-xs text-slate-500">
        <p>DocIntel • Phase 1 Multimodal Architecture with Go, Python, Redis & Postgres</p>
      </footer>
    </div>
  );
}
