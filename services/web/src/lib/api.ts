import { DocumentItem, ExtractionResults, UploadResponse } from "@/types/api";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/v1";
const DEFAULT_API_KEY = "dev-api-key-do-not-use-in-production";

export function getApiKey(): string {
  if (typeof window !== "undefined") {
    return localStorage.getItem("doc_intel_api_key") || DEFAULT_API_KEY;
  }
  return DEFAULT_API_KEY;
}

export function setApiKey(key: string) {
  if (typeof window !== "undefined") {
    localStorage.setItem("doc_intel_api_key", key);
  }
}

function getHeaders(extra?: HeadersInit): HeadersInit {
  return {
    Authorization: `Bearer ${getApiKey()}`,
    ...extra,
  };
}

export async function uploadDocument(file: File): Promise<UploadResponse> {
  const formData = new FormData();
  formData.append("file", file);

  const res = await fetch(`${API_BASE}/documents`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${getApiKey()}`,
    },
    body: formData,
  });

  if (!res.ok) {
    const errorData = await res.json().catch(() => ({}));
    throw new Error(errorData.error?.message || `Upload failed with HTTP ${res.status}`);
  }

  return res.json();
}

export async function getDocumentStatus(id: string): Promise<{ document_id: string; status: string; updated_at: string }> {
  const res = await fetch(`${API_BASE}/documents/${id}/status`, {
    headers: getHeaders(),
  });

  if (!res.ok) {
    throw new Error(`Failed to fetch status: ${res.statusText}`);
  }

  return res.json();
}

export async function getDocumentResults(id: string): Promise<ExtractionResults> {
  const res = await fetch(`${API_BASE}/documents/${id}/results`, {
    headers: getHeaders(),
  });

  if (!res.ok) {
    if (res.status === 404) {
      throw new Error("Results not ready yet");
    }
    throw new Error(`Failed to fetch results: ${res.statusText}`);
  }

  return res.json();
}

export async function listDocuments(status?: string): Promise<{ documents: DocumentItem[]; count: number }> {
  const url = new URL(`${API_BASE}/documents`);
  if (status) {
    url.searchParams.set("status", status);
  }

  const res = await fetch(url.toString(), {
    headers: getHeaders(),
  });

  if (!res.ok) {
    throw new Error(`Failed to list documents: ${res.statusText}`);
  }

  return res.json();
}

export async function getDocumentContentBlob(id: string): Promise<{ blob: Blob; contentType: string }> {
  const res = await fetch(`${API_BASE}/documents/${id}/content`, {
    headers: getHeaders(),
  });

  if (!res.ok) {
    throw new Error(`Failed to fetch document content: ${res.statusText}`);
  }

  const contentType = res.headers.get("Content-Type") || "application/octet-stream";
  const blob = await res.blob();
  return { blob, contentType };
}

