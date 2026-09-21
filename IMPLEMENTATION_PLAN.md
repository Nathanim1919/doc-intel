# doc-intel — Engineering Implementation Plan

> **Mission**: Build a production-grade document intelligence platform that takes uploaded
> documents, processes them asynchronously via VLMs, validates structured output, and
> delivers results with measurable confidence — starting with **bank cheques**.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Repository Layout](#repository-layout)
3. [Phase 1 — Build the Skeleton](#phase-1--build-the-skeleton)
4. [Phase 2 — Make AI Reliable](#phase-2--make-ai-reliable)
5. [Phase 3 — Make AI Measurable](#phase-3--make-ai-measurable)
6. [Phase 4 — Make It Production-Ready](#phase-4--make-it-production-ready)
7. [Cross-Phase Engineering Principles](#cross-phase-engineering-principles)
8. [Open Questions](#open-questions)

---

## Architecture Overview

```
Client (HTTP)
    │
    ▼
┌─────────────────────────────┐
│         Go API Service       │
│  Auth · Validation · Routes  │
│  Rate Limiting · Job Creation │
└──────────────┬──────────────┘
               │
   ┌───────────┼───────────┐
   ▼           ▼           ▼
PostgreSQL   Redis      Object
(metadata,   (job        Storage
 jobs,        queue)     (raw docs)
 results)
               │
               ▼
┌──────────────────────────────┐
│      Python Worker Service    │
│  Retrieval · Preprocessing    │
│  VLM · Parsing · Validation   │
│  Confidence · Persistence     │
└──────────────┬───────────────┘
               │
               ▼
           VLM API
        (Gemini / pluggable)
               │
               ▼
       Structured Extraction
               │
               ▼
       Evaluation / QA System
```

**Stack**: Go · Python · PostgreSQL · Redis · MinIO (local) / S3-compatible (prod) · Docker Compose · Gemini API

---

## Repository Layout

```
doc-intel/
│
├── services/
│   │
│   ├── api/                          # Go API service
│   │   ├── cmd/
│   │   │   └── server/
│   │   │       └── main.go
│   │   ├── internal/
│   │   │   ├── api/                  # HTTP handlers & routes
│   │   │   │   ├── routes.go
│   │   │   │   ├── handler_document.go
│   │   │   │   └── handler_job.go
│   │   │   ├── auth/                 # JWT / API key middleware
│   │   │   ├── document/             # Document domain + state machine
│   │   │   │   ├── service.go
│   │   │   │   └── state.go
│   │   │   ├── job/                  # Job creation & status
│   │   │   ├── storage/              # Object storage client
│   │   │   ├── queue/                # Redis queue client
│   │   │   └── db/                   # PostgreSQL models & queries
│   │   ├── go.mod
│   │   └── Dockerfile
│   │
│   └── worker/                       # Python worker service
│       ├── worker/
│       │   ├── main.py
│       │   ├── queue/
│       │   │   └── consumer.py       # Redis BRPOP + ack + dead-letter
│       │   ├── pipeline/
│       │   │   ├── loader.py         # Download from object storage
│       │   │   ├── preprocessor.py   # Image normalization, PDF→image
│       │   │   ├── extractor.py      # VLM call → raw output
│       │   │   ├── parser.py         # Raw JSON → Pydantic model
│       │   │   ├── validator.py      # Schema + domain validation
│       │   │   ├── confidence.py     # Per-field confidence scoring
│       │   │   ├── review.py         # Auto-approve vs human-review gate
│       │   │   ├── persister.py      # Write to DB
│       │   │   └── schemas/
│       │   │       └── cheque.py     # Bank cheque Pydantic schema
│       │   ├── vlm/
│       │   │   ├── base.py           # VisionModel Protocol (interface)
│       │   │   ├── gemini.py         # GeminiVisionModel implementation
│       │   │   └── prompts/
│       │   │       ├── cheque_v1.txt
│       │   │       └── cheque_v2.txt
│       │   └── observability/
│       │       └── logger.py         # Structured logging (structlog)
│       ├── pyproject.toml
│       └── Dockerfile
│
├── eval/                             # Evaluation system
│   ├── dataset/
│   │   ├── documents/                # Raw cheque images/PDFs
│   │   └── ground_truth/             # Manually verified JSON labels
│   ├── runner/
│   │   ├── run_eval.py               # Main evaluation entry point
│   │   ├── compare_experiments.py    # Diff two experiment results
│   │   └── regression_gate.py        # CI regression check
│   ├── metrics/
│   │   └── compute.py                # Field accuracy, doc accuracy, etc.
│   ├── reports/                      # Generated evaluation reports
│   └── experiments/
│       ├── registry.jsonl            # Append-only experiment log
│       └── baseline.json             # Pinned baseline for regression gate
│
├── migrations/                       # SQL migration files (numbered)
│   └── 001_initial_schema.sql
│
├── deploy/
│   ├── docker-compose.yml
│   ├── .env.example
│   └── Makefile
│
├── scripts/
│   ├── chaos/                        # Failure injection scripts
│   └── load/                         # Load testing scripts
│
├── docs/
│   └── adr/                          # Architecture Decision Records
│
├── IMPLEMENTATION_PLAN.md            # This file
└── README.md
```

---

## Phase 1 — Build the Skeleton

> **Theme**: System boundaries. Get the plumbing right before the AI.
> **Duration**: Week 1

### Goal

An end-to-end data path where a document can be uploaded, persisted to object storage,
recorded in PostgreSQL, enqueued in Redis, and consumed by the Python worker.

**No VLM yet** — the worker stub simply marks the job as `COMPLETED`.

---

### 1.1 Infrastructure — Docker Compose

**File**: `deploy/docker-compose.yml`

Services:

| Service    | Image              | Port   | Purpose                    |
|------------|--------------------|--------|----------------------------|
| `api`      | `./services/api`   | 8080   | Go HTTP API                |
| `worker`   | `./services/worker`| —      | Python async worker         |
| `postgres` | `postgres:16`      | 5432   | Metadata, jobs, results     |
| `redis`    | `redis:7`          | 6379   | Job queue                  |
| `minio`    | `minio/minio`      | 9000   | Object storage (local S3)  |

Rules:
- All secrets injected via `.env` (never baked into images)
- Health checks on all dependency services
- `api` and `worker` wait for `postgres` and `redis` to be healthy before starting

**File**: `deploy/.env.example`

```env
POSTGRES_DSN=postgres://docuser:secret@postgres:5432/docdb?sslmode=disable
REDIS_URL=redis://redis:6379/0
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=doc-intel
GEMINI_API_KEY=your-key-here
JWT_SECRET=replace-with-strong-secret
```

---

### 1.2 Database Schema

**File**: `migrations/001_initial_schema.sql`

#### Document Lifecycle State Machine

```
UPLOADED
    │
    ▼
QUEUED
    │
    ▼
PROCESSING
    │
    ├──────────────────┐
    ▼                  ▼
COMPLETED           FAILED
    │                  │
    ▼                  ▼
REVIEW_REQUIRED     RETRY (→ PROCESSING)
                       │
                       ▼ (after max retries)
                   FAILED_PERMANENTLY
```

#### Tables

```sql
-- Tenancy root
CREATE TABLE organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Auth principal
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    email           TEXT NOT NULL UNIQUE,
    api_key_hash    TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Document metadata + lifecycle
CREATE TABLE documents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users(id),
    filename    TEXT NOT NULL,
    mime_type   TEXT NOT NULL,
    storage_key TEXT NOT NULL,                  -- path in object storage
    file_hash   TEXT NOT NULL,                  -- SHA-256 for idempotency
    status      TEXT NOT NULL DEFAULT 'UPLOADED',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Async job tracking
CREATE TABLE processing_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id  UUID NOT NULL REFERENCES documents(id),
    status       TEXT NOT NULL DEFAULT 'QUEUED',
    attempt      INT NOT NULL DEFAULT 0,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Per-model-run metadata
CREATE TABLE extraction_runs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id    UUID NOT NULL REFERENCES documents(id),
    job_id         UUID NOT NULL REFERENCES processing_jobs(id),
    model          TEXT NOT NULL,
    model_version  TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    status         TEXT NOT NULL,
    latency_ms     INT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Field-level extracted values
CREATE TABLE extraction_fields (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    extraction_run_id UUID NOT NULL REFERENCES extraction_runs(id),
    field_name        TEXT NOT NULL,
    value             TEXT,
    raw_confidence    FLOAT,
    computed_confidence FLOAT,
    validation_status TEXT NOT NULL   -- PASSED | FAILED | SKIPPED
);
```

**Indexes**:
```sql
CREATE INDEX idx_documents_owner     ON documents(owner_id);
CREATE INDEX idx_documents_status    ON documents(status);
CREATE INDEX idx_documents_file_hash ON documents(file_hash);
CREATE INDEX idx_jobs_document       ON processing_jobs(document_id);
CREATE INDEX idx_fields_run          ON extraction_fields(extraction_run_id);
```

---

### 1.3 Go API Service

#### API Routes

```
POST   /v1/documents              → Upload document
GET    /v1/documents/:id          → Get document metadata
GET    /v1/documents/:id/status   → Get lifecycle status
GET    /v1/documents/:id/result   → Get extraction result
GET    /v1/documents              → List documents (with ?status= filter)
GET    /v1/jobs/:id               → Get job status
GET    /v1/health                 → Health check (no auth)
```

#### Document Upload Flow

```
1. Validate request (file size ≤ 10MB, MIME allowlist, filename sanitize)
2. Compute SHA-256 of file
3. Check documents table for existing file_hash + owner_id → 200 if exists (idempotent)
4. Upload file to object storage at key: documents/{document_id}/original.{ext}
5. INSERT into documents (status=UPLOADED)
6. INSERT into processing_jobs (status=QUEUED)
7. LPUSH job payload to Redis
8. UPDATE documents SET status=QUEUED
9. Return 202 Accepted { document_id, status }
```

#### Key Go packages

| Package | Purpose |
|---|---|
| `net/http` + `chi` or `stdlib` mux | HTTP router |
| `jackc/pgx` | PostgreSQL driver |
| `redis/go-redis` | Redis client |
| `minio/minio-go` | Object storage client |
| `golang-jwt/jwt` | JWT auth |
| `go.uber.org/zap` | Structured logging |

#### State Machine Guard (Go)

```go
// Valid transitions — any other transition is a bug, not a runtime error
var validTransitions = map[string][]string{
    "UPLOADED":           {"QUEUED"},
    "QUEUED":             {"PROCESSING"},
    "PROCESSING":         {"COMPLETED", "FAILED", "REVIEW_REQUIRED"},
    "FAILED":             {"RETRY", "FAILED_PERMANENTLY"},
    "RETRY":              {"PROCESSING"},
    "REVIEW_REQUIRED":    {"COMPLETED", "FAILED_PERMANENTLY"},
}

func (s *DocumentService) Transition(ctx context.Context, docID, from, to string) error {
    allowed := validTransitions[from]
    if !slices.Contains(allowed, to) {
        return fmt.Errorf("invalid transition %s → %s", from, to)
    }
    // atomic DB update + audit log
}
```

---

### 1.4 Python Worker Service (Phase 1 Stub)

**File**: `services/worker/worker/main.py`

```python
# Phase 1: consume job, mark PROCESSING, simulate work, mark COMPLETED
# No VLM — just proves the plumbing works end-to-end
```

**File**: `services/worker/worker/queue/consumer.py`

Responsibilities:
- `BRPOP` with 5s timeout (non-blocking loop, allows clean SIGTERM shutdown)
- Move job to `processing` set atomically (prevents duplicate pickup)
- Heartbeat: update job `started_at` and keep-alive ping every 10s
- On success: `LREM` from processing set, update DB
- On failure: retry counter check → requeue with backoff or dead-letter

```python
# Job visibility timeout pattern
# If worker crashes, a supervisor (or Redis TTL) returns the job to the queue
```

---

### Phase 1 Deliverable

```bash
# Upload a document
curl -X POST http://localhost:8080/v1/documents \
  -H "Authorization: Bearer <api-key>" \
  -F "file=@cheque.pdf"
# → 202 { "document_id": "abc-123", "status": "QUEUED" }

# Poll status
curl http://localhost:8080/v1/documents/abc-123/status
# → { "status": "COMPLETED" }  ← worker consumed job (stub, no VLM yet)
```

**Checklist**:
- [ ] Docker Compose brings up all 5 services cleanly
- [ ] File uploaded to MinIO, record in Postgres, job in Redis
- [ ] Worker consumes job, updates status to COMPLETED
- [ ] Invalid state transitions rejected by state machine guard
- [ ] `go test ./...` passes for state machine, storage, queue unit tests

---

---

## Phase 2 — Make AI Reliable

> **Theme**: AI inside a real system. Structured output + defensive engineering.
> **Duration**: Week 2

### Goal

The Python worker now calls a real VLM, parses structured output, validates it at multiple
layers, scores confidence per field, decides between auto-approval and human review,
and persists the full extraction result.

---

### 2.1 VLM Abstraction Layer

**File**: `services/worker/worker/vlm/base.py`

```python
from typing import Protocol, TypeVar
from pydantic import BaseModel

S = TypeVar("S", bound=BaseModel)

class VisionModel(Protocol):
    """All VLM implementations must satisfy this interface."""

    def extract(
        self,
        document_bytes: bytes,
        mime_type: str,
        schema: type[S],
        prompt_version: str,
    ) -> S:
        ...

    @property
    def model_name(self) -> str: ...

    @property
    def model_version(self) -> str: ...
```

**File**: `services/worker/worker/vlm/gemini.py`

```python
class GeminiVisionModel:
    """
    Wraps google-generativeai.
    - Loads prompt from prompts/{prompt_version}.txt
    - Uses Gemini JSON mode + Pydantic schema coercion
    - Retries 3x with exponential backoff + jitter
    - Hard timeout: 30s per call
    """
```

This means in Phase 3 you can run:

```python
model_a = GeminiVisionModel(model_name="gemini-1.5-flash")
model_b = GeminiVisionModel(model_name="gemini-1.5-pro")

result_a = model_a.extract(doc_bytes, "application/pdf", ChequeExtraction, "cheque_v2")
result_b = model_b.extract(doc_bytes, "application/pdf", ChequeExtraction, "cheque_v2")
# → compare without rewriting any pipeline code
```

---

### 2.2 Bank Cheque Extraction Schema

**File**: `services/worker/worker/pipeline/schemas/cheque.py`

```python
from decimal import Decimal
from datetime import date
from typing import Optional
from pydantic import BaseModel, field_validator

class ChequeExtraction(BaseModel):
    payee_name:     str
    amount_numeric: Decimal
    amount_words:   str
    date:           date
    account_number: str
    bank_name:      str
    branch_code:    Optional[str] = None
    memo:           Optional[str] = None

    @field_validator("amount_numeric")
    @classmethod
    def amount_must_be_positive(cls, v: Decimal) -> Decimal:
        if v <= 0:
            raise ValueError("Amount must be positive")
        return v

    @field_validator("date")
    @classmethod
    def date_must_be_realistic(cls, v: date) -> date:
        from datetime import date as dt
        if v > dt.today():
            raise ValueError("Cheque date cannot be in the future")
        if v.year < 1900:
            raise ValueError("Cheque date unrealistically old")
        return v
```

---

### 2.3 Extraction Pipeline

```
Document bytes (from object storage)
    │
    ▼  loader.py
Download & verify integrity
    │
    ▼  preprocessor.py
Normalize image (resize, deskew, enhance contrast)
    │
    ▼  extractor.py
Call VLM → raw model output (JSON string)
    │
    ▼  parser.py
Parse JSON → ChequeExtraction (Pydantic)
    │
    ▼  validator.py
Schema validation (already done by Pydantic)
Domain validation (amount_words ↔ amount_numeric, date sanity, account regex)
    │
    ▼  confidence.py
Per-field confidence scoring
    │
    ▼  review.py
Decision: COMPLETED (high confidence) or REVIEW_REQUIRED (low confidence)
    │
    ▼  persister.py
Write extraction_run + extraction_fields to PostgreSQL
Update document status
```

---

### 2.4 Domain Validation Rules

**File**: `services/worker/worker/pipeline/validator.py`

| Rule | Field(s) | Action on Failure |
|---|---|---|
| Amount words ↔ numeric (fuzzy match) | `amount_words`, `amount_numeric` | Mark field `FAILED` |
| Date ≤ today, year in [1900, 2030] | `date` | Mark field `FAILED` |
| Account number passes regex/Luhn | `account_number` | Mark field `FAILED` |
| Required fields present | all non-Optional | Mark document `REVIEW_REQUIRED` |
| ≥ 2 required fields fail | — | Force `REVIEW_REQUIRED` |

---

### 2.5 Confidence Scoring

**File**: `services/worker/worker/pipeline/confidence.py`

```python
@dataclass
class FieldConfidence:
    field_name:          str
    value:               Any
    raw_confidence:      float   # model-reported (0.0–1.0)
    validation_passed:   bool    # Pydantic schema passed
    domain_valid:        bool    # domain rules passed
    computed_confidence: float   # your heuristic score

def compute_confidence(
    raw: float,
    validation_passed: bool,
    domain_valid: bool,
) -> float:
    score = raw
    if not validation_passed:
        score *= 0.3
    elif not domain_valid:
        score *= 0.6
    return round(score, 4)
```

**Decision Gate**:

```
computed_confidence ≥ 0.85  →  COMPLETED (auto-approved)
computed_confidence < 0.85  →  REVIEW_REQUIRED
```

---

### 2.6 Failure Handling Matrix

| Scenario | Behavior |
|---|---|
| VLM timeout (>30s) | Retry with exponential backoff. Attempt 1→2→3, then dead-letter |
| Schema parse failure | Log raw VLM output. Mark FAILED. Retry once |
| Domain validation fails 1 field | Mark field FAILED. Continue. Apply confidence penalty |
| Domain validation fails ≥2 fields | Mark document REVIEW_REQUIRED |
| Worker crash mid-job | Job visibility timeout → job requeued from processing set |
| 3 consecutive failures | FAILED_PERMANENTLY. Emit alert log entry |

---

### Phase 2 Deliverable

```bash
curl -X POST http://localhost:8080/v1/documents \
  -H "Authorization: Bearer <api-key>" \
  -F "file=@cheque.pdf"
# → 202 { "document_id": "abc-123" }

# ~10 seconds later
curl http://localhost:8080/v1/documents/abc-123/result
# → {
#     "status": "COMPLETED",
#     "model": "gemini-1.5-flash",
#     "prompt_version": "cheque_v1",
#     "fields": {
#       "payee_name":     { "value": "Acme Corp",  "computed_confidence": 0.97 },
#       "amount_numeric": { "value": "125000.00",  "computed_confidence": 0.94 },
#       "amount_words":   { "value": "One hundred and twenty five thousand", "computed_confidence": 0.89 },
#       "date":           { "value": "2026-09-15", "computed_confidence": 0.99 }
#     }
#   }
```

**Checklist**:
- [ ] VLM called with correct prompt version
- [ ] Structured JSON extracted and validated
- [ ] Domain rules enforced (invalid amount → confidence penalty)
- [ ] Low-confidence documents routed to REVIEW_REQUIRED
- [ ] Worker crash → job auto-recovered from Redis
- [ ] `pytest services/worker/` passes for pipeline, validation, confidence unit tests

---

---

## Phase 3 — Make AI Measurable

> **Theme**: Evaluation. You can't improve what you can't measure.
> **Duration**: Week 3

### Goal

A complete evaluation harness: golden dataset, evaluation runner, field-level and
document-level metrics, experiment registry, and a regression gate that runs in CI.

---

### 3.1 Golden Dataset

**Directory**: `eval/dataset/`

```
eval/dataset/
├── documents/
│   ├── cheque_001.pdf          ← clear, digital
│   ├── cheque_002.jpg          ← clear, printed
│   ├── cheque_003.pdf          ← slightly skewed scan
│   ├── cheque_004.jpg          ← handwritten amount
│   ├── cheque_005.pdf          ← degraded / low quality
│   └── ...                     ← target: ≥ 20 documents
│
└── ground_truth/
    ├── cheque_001.json
    ├── cheque_002.json
    └── ...
```

Ground truth format:

```json
{
  "document_id": "cheque_001",
  "verified_by": "human",
  "verified_at": "2026-09-21T09:00:00Z",
  "fields": {
    "payee_name":     "Acme Corp",
    "amount_numeric": "125000.00",
    "amount_words":   "One hundred and twenty five thousand dollars",
    "date":           "2026-09-15",
    "account_number": "12345678",
    "bank_name":      "First National Bank"
  }
}
```

Document mix requirements:

| Type | Count | Purpose |
|---|---|---|
| Clear digital scan | 8 | Establishes upper-bound accuracy |
| Printed + scanned | 5 | Typical real-world quality |
| Slightly degraded | 4 | Tests robustness |
| Handwritten amount | 3 | Hard case — tests model limits |

---

### 3.2 Evaluation Runner

**File**: `eval/runner/run_eval.py`

```bash
python eval/runner/run_eval.py \
  --model gemini-1.5-flash \
  --prompt-version cheque_v2 \
  --dataset eval/dataset \
  --output eval/reports/exp_005.json
```

Runner loop:

```
For each document in dataset:
    1. Load document bytes
    2. Run extraction pipeline directly (bypass HTTP, bypass queue)
    3. Load ground truth JSON
    4. Compare predicted fields to ground truth
    5. Record per-field result: CORRECT | WRONG | MISSING | HALLUCINATED
    6. Accumulate into dataset-level metrics
Compute all metrics
Write report JSON
Append summary to experiments/registry.jsonl
```

---

### 3.3 Metrics

**File**: `eval/metrics/compute.py`

#### Quality Metrics

| Metric | Definition |
|---|---|
| `field_accuracy` | % of individual fields correctly extracted across all documents |
| `document_accuracy` | % of documents where ALL required fields are correct |
| `missing_field_rate` | % of fields the model omitted (returned null) |
| `hallucination_rate` | % of fields the model invented (not verifiable in source) |
| `invalid_field_rate` | % of fields failing domain validation |

#### Per-Field Breakdown

```json
{
  "payee_name":     { "accuracy": 0.95, "missing": 0.02, "wrong": 0.03 },
  "amount_numeric": { "accuracy": 0.91, "missing": 0.04, "wrong": 0.05 },
  "amount_words":   { "accuracy": 0.83, "missing": 0.05, "wrong": 0.12 },
  "date":           { "accuracy": 0.97, "missing": 0.01, "wrong": 0.02 }
}
```

This tells you: *amount_words is the hardest field — target it first in prompt engineering.*

#### Performance Metrics

| Metric | Definition |
|---|---|
| `vlm_latency_p50/p95/p99` | VLM call latency distribution |
| `end_to_end_latency_p50/p95` | Upload to result available |
| `queue_latency_avg` | Time spent in queue before pickup |

#### Reliability Metrics

| Metric | Definition |
|---|---|
| `failure_rate` | % of documents that reached FAILED_PERMANENTLY |
| `retry_rate` | % of documents that required ≥1 retry |
| `review_required_rate` | % routed to human review |

#### Economics

| Metric | Definition |
|---|---|
| `cost_per_document` | Total VLM API cost / documents processed |
| `cost_per_field` | cost_per_document / fields_per_document |

---

### 3.4 Experiment Registry

**File**: `eval/experiments/registry.jsonl` (append-only)

```jsonl
{"experiment_id":"exp_001","timestamp":"2026-09-21T10:00:00Z","model":"gemini-1.5-flash","prompt_version":"cheque_v1","dataset_version":"v1","field_accuracy":0.872,"document_accuracy":0.70,"missing_field_rate":0.06,"hallucination_rate":0.012,"vlm_latency_p95_ms":3100,"cost_per_doc_usd":0.0021,"notes":"Baseline"}
{"experiment_id":"exp_002","timestamp":"2026-09-22T10:00:00Z","model":"gemini-1.5-flash","prompt_version":"cheque_v2","dataset_version":"v1","field_accuracy":0.903,"document_accuracy":0.76,"missing_field_rate":0.04,"hallucination_rate":0.008,"vlm_latency_p95_ms":3200,"cost_per_doc_usd":0.0022,"notes":"Improved amount_words prompt"}
{"experiment_id":"exp_003","timestamp":"2026-09-23T10:00:00Z","model":"gemini-1.5-pro","prompt_version":"cheque_v2","dataset_version":"v1","field_accuracy":0.941,"document_accuracy":0.83,"missing_field_rate":0.02,"hallucination_rate":0.005,"vlm_latency_p95_ms":5800,"cost_per_doc_usd":0.0087,"notes":"Pro model — higher accuracy, higher cost"}
```

---

### 3.5 Experiment Comparison

**File**: `eval/runner/compare_experiments.py`

```bash
python eval/runner/compare_experiments.py exp_002 exp_003

Experiment Comparison: exp_002 → exp_003
─────────────────────────────────────────────────
field_accuracy       0.903 → 0.941   (+4.2%)  ✅
document_accuracy    0.76  → 0.83    (+9.2%)  ✅
hallucination_rate   0.008 → 0.005   (-37.5%) ✅
vlm_latency_p95_ms   3200  → 5800    (+81.3%) ⚠️
cost_per_doc_usd     0.0022 → 0.0087 (+295%)  ⚠️
─────────────────────────────────────────────────
Decision: Pro model is more accurate but 4× more expensive and 80% slower.
Use Flash for high-volume, Pro for high-value documents.
```

---

### 3.6 Regression Gate (CI)

**File**: `eval/runner/regression_gate.py`

```bash
# Runs automatically on every PR that changes prompts or schemas
python eval/runner/regression_gate.py \
  --baseline eval/experiments/baseline.json \
  --current  eval/reports/exp_006.json \
  --threshold 0.02    # Fail if field_accuracy drops > 2%

# → EXIT 0 if regression-free
# → EXIT 1 with diff if regression detected
```

---

### Phase 3 Deliverable

You can answer **"How good is this system?"** with numbers:

```
Experiment exp_003 — gemini-1.5-pro / cheque_v2
─────────────────────────────────────────────────
Field accuracy:       94.1%
Document accuracy:    83.0%
Missing fields:        2.0%
Hallucinations:        0.5%
VLM latency p95:    5,800ms
Review required:      12.0%
Cost per document:  $0.0087

Weakest field: amount_words (79% accuracy) → next prompt iteration target
```

**Checklist**:
- [ ] ≥ 20 documents in golden dataset, all manually verified
- [ ] Eval runner produces reproducible metric reports
- [ ] Experiment registry has ≥ 3 experiments comparing prompt/model combinations
- [ ] Regression gate catches a deliberate accuracy drop
- [ ] `compare_experiments.py` shows clear trade-off analysis

---

---

## Phase 4 — Make It Production-Ready

> **Theme**: Deployment + ownership. Can a real customer actually use this?
> **Duration**: Week 4

### Goal

Containerized production deployment with TLS, structured distributed tracing,
security hardening, failure/load testing, and first real user feedback.

---

### 4.1 Observability

Every request must be traceable from client → API → queue → worker → VLM → result.

#### Trace Context Propagation

```
request_id: req_839a
  │
  ├── Go API
  │     document_id: doc_abc1
  │     job_id:      job_xyz2
  │     → enqueued with request_id in job payload
  │
  └── Python Worker
        request_id:  req_839a   ← propagated from job payload
        worker_id:   worker-02
        vlm_call_id: gemini_req_f4a9
        → every log line includes all four IDs
```

#### Structured Log Format (Go + Python)

```json
{
  "ts":          "2026-09-28T12:34:56.789Z",
  "level":       "info",
  "msg":         "extraction completed",
  "request_id":  "req_839a",
  "document_id": "doc_abc1",
  "job_id":      "job_xyz2",
  "vlm_latency_ms": 3241,
  "field_count": 8,
  "confidence_min": 0.81
}
```

Debugging a failed document becomes:

```bash
grep '"document_id":"doc_abc1"' /var/log/api.log
grep '"document_id":"doc_abc1"' /var/log/worker.log
# → complete trace across both services
```

#### Metrics Endpoint

`GET /metrics` → Prometheus text format

Key metrics exposed:
- `docapi_requests_total{method,path,status}`
- `docapi_request_duration_seconds{method,path}`
- `docapi_queue_depth`
- `docworker_jobs_processed_total{status}`
- `docworker_vlm_latency_seconds{model,prompt_version}`

---

### 4.2 Security Hardening

#### Upload Security (Go API)

```go
const (
    MaxFileSize    = 10 * 1024 * 1024  // 10 MB
    AllowedMIMEs   = ["application/pdf", "image/jpeg", "image/png"]
)

func validateUpload(r *http.Request) error {
    // 1. Check Content-Length before reading body
    // 2. Read with io.LimitReader(body, MaxFileSize+1) to detect oversize
    // 3. Sniff actual MIME from first 512 bytes (not header)
    // 4. Sanitize filename: strip path traversal, non-printables
}
```

#### Authorization

```go
// Every document endpoint enforces ownership:
// SELECT id FROM documents WHERE id=$1 AND owner_id=$2
// → 404 if not found (not 403 — don't leak existence)
```

#### Rate Limiting

```go
// Token bucket per user:
// - 100 uploads/hour
// - 1000 GET requests/hour
// Headers: X-RateLimit-Remaining, X-RateLimit-Reset
```

#### Secrets

- API keys: stored as bcrypt hash in DB, never logged
- Gemini API key: injected via Docker secret or environment at runtime
- Postgres credentials: never in source control
- MinIO credentials: rotate before production

---

### 4.3 Production Deployment

**File**: `deploy/docker-compose.yml` (production variant)

```yaml
services:
  api:
    image: ghcr.io/yourorg/doc-intel-api:latest
    restart: always
    deploy:
      resources:
        limits:
          cpus: "1.0"
          memory: 512M
    environment:
      - POSTGRES_DSN=${POSTGRES_DSN}
      - REDIS_URL=${REDIS_URL}
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/v1/health"]
      interval: 10s
      retries: 3

  worker:
    image: ghcr.io/yourorg/doc-intel-worker:latest
    restart: always
    deploy:
      replicas: 2           # Two workers for basic redundancy
      resources:
        limits:
          cpus: "1.0"
          memory: 1G
```

**File**: `deploy/Makefile`

```makefile
dev-up:
	docker compose up -d

dev-down:
	docker compose down

migrate:
	go run ./services/api/cmd/migrate

run-eval:
	python eval/runner/run_eval.py \
	  --model gemini-1.5-flash \
	  --prompt-version cheque_v2 \
	  --dataset eval/dataset \
	  --output eval/reports/latest.json

test:
	cd services/api && go test ./...
	cd services/worker && pytest

regression-gate:
	python eval/runner/regression_gate.py \
	  --baseline eval/experiments/baseline.json \
	  --current  eval/reports/latest.json

build:
	docker compose build

push:
	docker compose push
```

---

### 4.4 Failure Testing

**Directory**: `scripts/chaos/`

| Script | What it tests |
|---|---|
| `kill_worker.sh` | Kill worker mid-job → verify job auto-requeued |
| `fill_queue.sh` | Push 100 jobs rapidly → verify no queue corruption |
| `corrupt_payload.sh` | Push malformed JSON job → verify dead-letter behavior |
| `storage_down.sh` | Take MinIO offline → verify API returns 503, not 500 |
| `db_disconnect.sh` | Disconnect Postgres → verify API returns 503 gracefully |

Each script:
1. Injects the failure
2. Makes assertions (via curl / db query)
3. Removes the failure
4. Verifies system self-heals

---

### 4.5 Load Testing

**File**: `scripts/load/load_test.py`

```bash
python scripts/load/load_test.py \
  --url http://api.yourdomain.com \
  --concurrency 50 \
  --documents 200

Results:
  Successful uploads:    198 / 200 (99.0%)
  API p50 latency:       124ms
  API p95 latency:       891ms
  Queue depth peak:      47
  Worker throughput:     8.3 docs/min
  Error rate:            1.0%
```

Success criteria: ≤5% error rate at 50 concurrent users.

---

### 4.6 First Customer / User Feedback

Deployment target: **Hetzner VPS** or **Fly.io** (cheap, real TLS, real IP)

Steps:
1. Deploy with a real domain + Let's Encrypt TLS
2. Issue API keys to 2–3 early users (colleagues, finance contacts)
3. Have them upload real cheques via the API
4. Collect failure cases → add to golden dataset (closes Phase 3 loop)
5. Track: which fields fail most, what document quality breaks the system

This is the **customer discovery + technical validation** loop:

```
Real user uploads real document
    ↓
System extracts fields
    ↓
User confirms or corrects
    ↓
Failures → golden dataset
    ↓
Eval runner → metrics improve
    ↓
Better system → better user experience
```

---

### Phase 4 Deliverable

You can answer **"Can someone actually use this?"** with:

```
✅ Deployed to production (HTTPS, valid TLS)
✅ Structured logs with full trace IDs
✅ Worker crash → job auto-recovered (tested)
✅ All chaos tests passed
✅ Load test: 50 concurrent users, 1% error rate
✅ 3 real users processed real cheques
✅ Failure cases fed back into golden dataset
```

---

---

## Cross-Phase Engineering Principles

| Principle | How We Apply It |
|---|---|
| **State machines** | Document lifecycle is explicit. No direct DB updates bypassing transition guards |
| **Idempotency** | Same file hash + same owner → return existing document. Same job → one extraction |
| **At-least-once delivery** | Jobs requeued if worker crashes. Idempotency prevents double-processing |
| **Abstraction at boundaries** | VLM behind an interface. Storage behind an interface. Swap implementations without pipeline changes |
| **Versioning** | Prompts, schemas, and models are all versioned. Eval always knows what produced what |
| **Observability first** | `request_id` propagated through every component from day 1, not added later |
| **Fail safely** | Low confidence → human review, never silent auto-approval |
| **Golden dataset is sacred** | No model/prompt change ships without running the eval suite |
| **Cost awareness** | `cost_per_document` tracked from Phase 3. Architecture decisions account for it |

---

## Open Questions

> These must be answered before starting Phase 1.

1. **Object Storage**: Local dev = MinIO. Production = AWS S3 / GCP GCS / Cloudflare R2?
   The storage client interface is the same; only the concrete implementation changes.

2. **VLM Model**: Starting with Gemini. Do you have a Gemini API key?
   Recommendation: Flash for eval baseline (cheaper), Pro as comparison target in Phase 3.

3. **Auth Strategy**: API key (simpler, Phase 1 appropriate) vs JWT (more production-realistic)?
   Recommendation: API key in Phase 1–3, JWT in Phase 4 when deploying to real users.

4. **Human Review UI**: Phase 1–3 exposes review via API (`GET /v1/documents?status=REVIEW_REQUIRED`).
   A minimal review frontend can be added in Phase 4 or as a separate workstream.

5. **Cloud Deployment Target**: Hetzner VPS / Fly.io / Render / GCP Cloud Run?
   This only affects Phase 4. Recommendation: Fly.io (simple Docker deploy, free TLS, cheap).
