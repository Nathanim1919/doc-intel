"use client";

import React, { useState } from "react";
import { Key, Check, Shield } from "lucide-react";
import { getApiKey, setApiKey } from "@/lib/api";

export function Header() {
  const [showKeyModal, setShowKeyModal] = useState(false);
  const [apiKeyInput, setApiKeyInput] = useState(getApiKey());
  const [saved, setSaved] = useState(false);

  const handleSave = () => {
    setApiKey(apiKeyInput);
    setSaved(true);
    setTimeout(() => {
      setSaved(false);
      setShowKeyModal(false);
    }, 1000);
  };

  return (
    <header className="border-b border-slate-800/80 bg-slate-950/40 backdrop-blur-xl sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="h-9 w-9 rounded-xl bg-gradient-to-tr from-indigo-600 via-indigo-500 to-cyan-400 flex items-center justify-center shadow-lg shadow-indigo-500/20">
            <span className="font-bold text-white tracking-wider text-base">DI</span>
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-bold text-lg text-white tracking-tight">DocIntel</span>
              <span className="text-[10px] uppercase font-bold tracking-wider px-2 py-0.5 rounded-full bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
                Phase 1 Live
              </span>
            </div>
          </div>
        </div>

        <nav className="flex items-center gap-4">
          <button
            onClick={() => setShowKeyModal(true)}
            className="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-slate-700/80 bg-slate-900/60 hover:bg-slate-800 text-xs font-medium text-slate-300 transition-all shadow-sm"
          >
            <Key className="w-3.5 h-3.5 text-indigo-400" />
            <span>API Key</span>
          </button>
          <a
            href="https://github.com/Nathanim1919/doc-intel"
            target="_blank"
            rel="noreferrer"
            className="text-xs text-slate-400 hover:text-white transition-colors"
          >
            GitHub
          </a>
        </nav>
      </div>

      {showKeyModal && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 w-full max-w-md shadow-2xl space-y-4">
            <div className="flex items-center gap-3 text-white">
              <div className="p-2 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
                <Shield className="w-5 h-5" />
              </div>
              <div>
                <h3 className="font-semibold text-base">Set DocIntel API Key</h3>
                <p className="text-xs text-slate-400">Default dev key is preloaded for local testing.</p>
              </div>
            </div>

            <input
              type="text"
              value={apiKeyInput}
              onChange={(e) => setApiKeyInput(e.target.value)}
              placeholder="Paste Bearer token here"
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-xs text-slate-200 font-mono focus:outline-none focus:border-indigo-500/50"
            />

            <div className="flex items-center justify-end gap-2 pt-2">
              <button
                onClick={() => setShowKeyModal(false)}
                className="px-3 py-2 text-xs font-medium text-slate-400 hover:text-slate-200"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-xs font-semibold text-white shadow-md shadow-indigo-600/30 transition-all"
              >
                {saved ? <Check className="w-4 h-4" /> : null}
                {saved ? "Saved" : "Save Key"}
              </button>
            </div>
          </div>
        </div>
      )}
    </header>
  );
}
