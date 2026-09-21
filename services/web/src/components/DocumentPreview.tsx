"use client";

import React, { useEffect, useState } from "react";
import { Eye, FileText, Loader2, Download, ZoomIn, ZoomOut } from "lucide-react";
import { getDocumentContentBlob } from "@/lib/api";

interface DocumentPreviewProps {
  documentId: string;
}

export function DocumentPreview({ documentId }: DocumentPreviewProps) {
  const [objectUrl, setObjectUrl] = useState<string | null>(null);
  const [contentType, setContentType] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [zoom, setZoom] = useState<number>(100);

  useEffect(() => {
    let activeUrl: string | null = null;
    let isMounted = true;

    async function loadContent() {
      try {
        setLoading(true);
        setError(null);
        setZoom(100);
        const { blob, contentType: ct } = await getDocumentContentBlob(documentId);
        if (!isMounted) return;

        setContentType(ct);
        activeUrl = URL.createObjectURL(blob);
        setObjectUrl(activeUrl);
      } catch (err: unknown) {
        if (!isMounted) return;
        setError(err instanceof Error ? err.message : "Failed to load document content");
      } finally {
        if (isMounted) setLoading(false);
      }
    }

    loadContent();

    return () => {
      isMounted = false;
      if (activeUrl) {
        URL.revokeObjectURL(activeUrl);
      }
    };
  }, [documentId]);

  const isPdf = contentType.includes("pdf");
  const isImage = contentType.startsWith("image/");

  return (
    <div className="bg-slate-950/70 border border-slate-800 rounded-3xl p-5 shadow-2xl flex flex-col h-[640px]">
      <div className="flex items-center justify-between pb-3 mb-3 border-b border-slate-800/80">
        <div className="flex items-center gap-2">
          <Eye className="w-4 h-4 text-indigo-400" />
          <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300">
            Source Document Preview
          </h3>
        </div>

        {objectUrl && (
          <div className="flex items-center gap-1">
            {isImage && (
              <>
                <button
                  onClick={() => setZoom((z) => Math.max(z - 20, 40))}
                  className="p-1 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-white"
                  title="Zoom Out"
                >
                  <ZoomOut className="w-3.5 h-3.5" />
                </button>
                <span className="text-[10px] font-mono text-slate-500 w-9 text-center">{zoom}%</span>
                <button
                  onClick={() => setZoom((z) => Math.min(z + 20, 200))}
                  className="p-1 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-white"
                  title="Zoom In"
                >
                  <ZoomIn className="w-3.5 h-3.5" />
                </button>
              </>
            )}
            <a
              href={objectUrl}
              download={`document-${documentId.slice(0, 8)}`}
              className="p-1 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-white ml-2"
              title="Download raw file"
            >
              <Download className="w-3.5 h-3.5" />
            </a>
          </div>
        )}
      </div>

      <div className="flex-1 overflow-auto rounded-2xl bg-slate-900/40 border border-slate-800/60 flex items-center justify-center p-3 relative">
        {loading && (
          <div className="flex flex-col items-center gap-2 text-slate-400 text-xs">
            <Loader2 className="w-6 h-6 animate-spin text-indigo-400" />
            <span>Streaming original file from MinIO...</span>
          </div>
        )}

        {error && (
          <div className="text-center text-xs text-red-400 space-y-1">
            <p>Could not preview document</p>
            <p className="text-[11px] text-slate-500 font-mono">{error}</p>
          </div>
        )}

        {!loading && !error && objectUrl && (
          <>
            {isPdf ? (
              <iframe
                src={`${objectUrl}#toolbar=0`}
                className="w-full h-full rounded-xl border-0"
                title="Document PDF preview"
              />
            ) : isImage ? (
              <div className="w-full h-full overflow-auto flex items-center justify-center">
                <img
                  src={objectUrl}
                  alt="Source document preview"
                  style={{ width: `${zoom}%` }}
                  className="rounded-lg object-contain max-h-none shadow-lg transition-all"
                />
              </div>
            ) : (
              <div className="text-center text-xs text-slate-400 space-y-2">
                <FileText className="w-10 h-10 mx-auto text-slate-500" />
                <p>Preview format ({contentType}) cannot be rendered directly in-browser.</p>
                <a
                  href={objectUrl}
                  download
                  className="inline-block px-3 py-1.5 rounded-lg bg-indigo-600 text-white text-xs font-semibold"
                >
                  Download File
                </a>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
