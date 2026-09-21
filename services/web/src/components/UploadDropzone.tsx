"use client";

import React, { useRef, useState } from "react";
import { UploadCloud, FileText, AlertCircle, Loader2 } from "lucide-react";
import { uploadDocument } from "@/lib/api";

interface UploadDropzoneProps {
  onUploaded: (documentId: string, initialStatus: string) => void;
}

export function UploadDropzone({ onUploaded }: UploadDropzoneProps) {
  const [isDragging, setIsDragging] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFile = async (file: File) => {
    setError(null);

    // MIME check: PDF, PNG, JPEG, TIFF
    const validTypes = ["application/pdf", "image/png", "image/jpeg", "image/tiff"];
    if (!validTypes.includes(file.type)) {
      setError("Please upload a supported document (PDF, PNG, JPG, or TIFF).");
      return;
    }

    if (file.size > 10 * 1024 * 1024) {
      setError("File exceeds the 10 MB size limit.");
      return;
    }

    try {
      setIsUploading(true);
      const res = await uploadDocument(file);
      onUploaded(res.document_id, res.status);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to upload document");
    } finally {
      setIsUploading(false);
    }
  };

  return (
    <div className="w-full">
      <div
        onDragOver={(e) => {
          e.preventDefault();
          setIsDragging(true);
        }}
        onDragLeave={() => setIsDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setIsDragging(false);
          if (e.dataTransfer.files?.[0]) {
            handleFile(e.dataTransfer.files[0]);
          }
        }}
        onClick={() => fileInputRef.current?.click()}
        className={`group relative cursor-pointer border-2 border-dashed rounded-3xl p-8 transition-all flex flex-col items-center justify-center text-center overflow-hidden ${
          isDragging
            ? "border-indigo-500 bg-indigo-500/5 shadow-2xl shadow-indigo-500/10 scale-[1.01]"
            : "border-slate-800 hover:border-slate-700 bg-slate-950/40 hover:bg-slate-900/30"
        }`}
      >
        <input
          ref={fileInputRef}
          type="file"
          accept=".pdf,.png,.jpg,.jpeg,.tiff"
          className="hidden"
          onChange={(e) => {
            if (e.target.files?.[0]) {
              handleFile(e.target.files[0]);
            }
          }}
        />

        {/* Subtle background glow */}
        <div className="absolute inset-0 bg-radial from-indigo-500/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />

        <div className="h-16 w-16 rounded-2xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 flex items-center justify-center mb-4 group-hover:scale-110 transition-transform shadow-inner">
          {isUploading ? (
            <Loader2 className="w-8 h-8 animate-spin" />
          ) : (
            <UploadCloud className="w-8 h-8" />
          )}
        </div>

        <div className="space-y-1">
          <p className="text-sm font-semibold text-slate-200">
            {isUploading ? "Uploading & Enqueueing for Gemini..." : "Drop invoice, receipt, or passport"}
          </p>
          <p className="text-xs text-slate-400">
            Supports PDF, PNG, JPG, TIFF up to 10 MB
          </p>
        </div>

        <div className="mt-4 flex items-center gap-2">
          <span className="text-[11px] px-2.5 py-1 rounded-full bg-slate-800/80 text-slate-300 font-mono">
            Async Queue (Redis)
          </span>
          <span className="text-[11px] px-2.5 py-1 rounded-full bg-slate-800/80 text-slate-300 font-mono">
            Gemini Multimodal
          </span>
        </div>
      </div>

      {error && (
        <div className="mt-3 p-3 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}
    </div>
  );
}
