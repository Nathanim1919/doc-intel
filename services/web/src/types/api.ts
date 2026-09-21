export interface DocumentItem {
  id: string;
  filename: string;
  mime_type: string;
  size_bytes: number;
  status: "UPLOADED" | "QUEUED" | "PROCESSING" | "COMPLETED" | "REVIEW_REQUIRED" | "FAILED" | "FAILED_PERMANENTLY";
  created_at: string;
  updated_at: string;
}

export interface ExtractionRun {
  id: string;
  job_id: string;
  model: string;
  model_version?: string;
  prompt_version: string;
  status: string;
  latency_ms?: number;
  tokens_used?: number;
  error?: string;
  created_at: string;
}

export interface ExtractionField {
  id: string;
  field_name: string;
  value?: string;
  raw_confidence?: number;
  computed_confidence?: number;
  validation_status: "PASSED" | "FAILED" | "SKIPPED";
  page_number?: number;
}

export interface ExtractionResults {
  document_id: string;
  run: ExtractionRun;
  fields: ExtractionField[];
}

export interface UploadResponse {
  document_id: string;
  job_id?: string;
  status: string;
  is_duplicate: boolean;
}
