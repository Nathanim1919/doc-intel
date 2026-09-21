# doc-intel

Production-oriented document intelligence pipeline using Go, Python, VLMs, Redis, PostgreSQL, object storage, and automated evaluation.







Yes. Before writing more daily tasks, I think we should **freeze the architecture and the month-level engineering phases**.

The key is that `doc-intel` should not become "a VLM wrapper." It should be a **small production AI system** where every component exists for a reason.

# `doc-intel` — Architectural Blueprint

## 1. What are we actually building?

A document-intelligence platform that takes an uploaded document, processes it asynchronously, extracts structured information using a VLM, validates the result, assigns confidence, and makes the result available for downstream systems or human review.

The first document type should be **one narrowly defined document class**, not "all documents."

For example:

```text
Bank check
```

Then:

```text
Image/PDF
    ↓
Upload
    ↓
Document record
    ↓
Async processing job
    ↓
VLM extraction
    ↓
Structured JSON
    ↓
Schema validation
    ↓
Domain validation
    ↓
Confidence assessment
    ↓
Human review if necessary
    ↓
Persist result
    ↓
API
    ↓
Frontend / external client
```

The important part is that **AI is only one component of the system**.

---

# 2. High-Level Architecture

```text
                         ┌───────────────────┐
                         │      Client       │
                         │ Web / API Client  │
                         └─────────┬─────────┘
                                   │
                                   │ HTTPS
                                   ▼
                         ┌───────────────────┐
                         │      Go API       │
                         │                   │
                         │ Auth              │
                         │ Validation        │
                         │ Rate limiting     │
                         │ Request handling  │
                         └───────┬───────────┘
                                 │
               ┌─────────────────┼─────────────────┐
               │                 │                 │
               ▼                 ▼                 ▼
        ┌────────────┐    ┌────────────┐    ┌────────────┐
        │ PostgreSQL │    │   Object   │    │   Redis    │
        │            │    │  Storage   │    │   Queue    │
        │ Metadata   │    │            │    │            │
        │ Jobs       │    │ Raw docs   │    │ Jobs       │
        │ Results    │    │            │    │            │
        └────────────┘    └────────────┘    └─────┬──────┘
                                                  │
                                                  │ Job
                                                  ▼
                                        ┌──────────────────┐
                                        │  Python Worker   │
                                        │                  │
                                        │ Retrieval        │
                                        │ Preprocessing    │
                                        │ VLM invocation   │
                                        │ Parsing          │
                                        │ Validation       │
                                        │ Confidence       │
                                        └────────┬─────────┘
                                                 │
                                                 ▼
                                        ┌──────────────────┐
                                        │       VLM        │
                                        │                  │
                                        │ Gemini / other   │
                                        └────────┬─────────┘
                                                 │
                                                 ▼
                                        Structured Extraction
                                                 │
                                                 ▼
                                        ┌──────────────────┐
                                        │ Evaluation / QA  │
                                        └──────────────────┘
```

---

# 3. Why Go + Python?

This is intentional.

## Go

Go owns the **system**.

```text
Go
├── HTTP API
├── authentication
├── request validation
├── rate limiting
├── document lifecycle
├── job creation
├── Redis interaction
├── database interaction
└── API responses
```

This demonstrates:

* backend architecture
* concurrency
* networking
* reliability
* distributed systems
* API design

---

## Python

Python owns the **AI workload**.

```text
Python
├── VLM interaction
├── document preprocessing
├── structured extraction
├── Pydantic validation
├── domain validation
├── confidence processing
└── evaluation
```

This demonstrates:

* AI engineering
* multimodal models
* evaluation
* data processing
* ML/LLM integration

The boundary between them is itself an engineering problem.

---

# 4. Service Boundaries

I would start with only **two application services**.

### Service 1 — API

```text
/services/api
```

Go.

Responsibilities:

* accept requests
* authenticate
* upload documents
* create database records
* enqueue jobs
* expose results
* expose job status

---

### Service 2 — Worker

```text
/services/worker
```

Python.

Responsibilities:

* consume jobs
* retrieve document
* call VLM
* validate output
* calculate/store confidence
* update processing state

Don't create 8 microservices just to look "senior."

**Two services are enough.**

The sophistication comes from the architecture inside them.

---

# 5. Document Lifecycle

Every document should have a lifecycle.

```text
UPLOADED
    │
    ▼
QUEUED
    │
    ▼
PROCESSING
    │
    ├───────────────┐
    ▼               ▼
COMPLETED         FAILED
    │               │
    │               ▼
    │             RETRY
    │               │
    │               ▼
    │          PROCESSING
    │
    ▼
REVIEW_REQUIRED
```

Eventually:

```text
COMPLETED
REVIEW_REQUIRED
FAILED_PERMANENTLY
```

This forces you to think about **state machines**, rather than just endpoints.

---

# 6. Database Architecture

Initial schema:

```text
organizations
    │
    └── users

documents
    │
    ├── document_versions
    │
    ├── processing_jobs
    │
    └── extraction_runs
            │
            └── extraction_fields
```

A simplified version:

```text
documents
---------
id
owner_id
filename
mime_type
storage_key
status
created_at
updated_at
```

```text
processing_jobs
---------------
id
document_id
status
attempt
started_at
completed_at
error
created_at
```

```text
extraction_runs
---------------
id
document_id
model
model_version
prompt_version
status
latency_ms
created_at
```

```text
extraction_fields
-----------------
id
extraction_run_id
field_name
value
confidence
validation_status
```

This becomes extremely useful later for evaluation and regression analysis.

---

# 7. Object Storage

Never store the actual document binary inside PostgreSQL.

Instead:

```text
PostgreSQL

document
 └── storage_key
```

and:

```text
Object Storage

documents/
    └── {document_id}/
          └── original.pdf
```

The application only stores metadata and references.

This teaches you a fundamental production architecture pattern:

> **Database for state, object storage for large blobs.**

---

# 8. Redis Queue

This isn't simply:

```text
LPUSH
BRPOP
```

The queue should eventually support:

```text
queued
processing
completed
failed
retrying
dead-letter
```

Think about:

### Idempotency

What if the same job arrives twice?

```text
Job A
Job A
```

The extraction shouldn't accidentally happen twice.

---

### Retry

```text
attempt 1
    ↓
failure
    ↓
backoff
    ↓
attempt 2
    ↓
failure
    ↓
attempt 3
    ↓
dead letter
```

---

### Worker crash

Imagine:

```text
Worker
  ↓
gets job
  ↓
CRASH
```

What happens to the job?

This is where you'll start learning real distributed-systems concepts.

---

# 9. VLM Architecture

The Python worker should **not** directly scatter provider-specific code everywhere.

Use an abstraction:

```text
VisionModel
    │
    ├── GeminiVisionModel
    │
    └── OtherVisionModel
```

Conceptually:

```python
result = model.extract(
    document=document,
    schema=CheckExtraction
)
```

This means later you can benchmark:

```text
Model A
vs
Model B
```

without rewriting your worker.

---

# 10. Extraction Pipeline

This is the heart of the AI system.

```text
Document
   │
   ▼
Load
   │
   ▼
Preprocess
   │
   ▼
VLM
   │
   ▼
Raw Model Output
   │
   ▼
Parse
   │
   ▼
Pydantic Schema
   │
   ▼
Domain Validation
   │
   ▼
Confidence
   │
   ▼
Review Decision
   │
   ▼
Persist
```

---

# 11. Schema Validation

Suppose the VLM produces:

```json
{
  "amount": "125000",
  "date": "2026-09-15",
  "account_number": "123456789"
}
```

Your system doesn't simply trust it.

It asks:

```text
Is amount valid?
Is date valid?
Is account number valid?
Are required fields present?
Are values the expected types?
```

So:

```text
AI output
   ↓
Syntax validation
   ↓
Schema validation
   ↓
Domain validation
```

This is an important distinction:

> **The model generates. Your software decides whether the result is acceptable.**

---

# 12. Confidence

Each field can have:

```json
{
  "value": "125000",
  "confidence": 0.94
}
```

But we should be intellectually honest here.

Initially, that is **model-reported confidence**, not necessarily statistically calibrated confidence.

Later, we can investigate calibration.

Then:

```text
High confidence
      ↓
Automatic processing

Low confidence
      ↓
Human review
```

This is much closer to how real AI systems should operate.

---

# 13. Evaluation System

This should become one of the most important parts of the repository.

```text
eval/
├── dataset/
│   ├── documents/
│   └── ground_truth/
│
├── runner/
├── metrics/
├── reports/
└── experiments/
```

The basic loop:

```text
Golden Dataset
      │
      ▼
Run Model
      │
      ▼
Predictions
      │
      ▼
Compare Ground Truth
      │
      ▼
Metrics
      │
      ▼
Report
```

---

# 14. What We Measure

Not just:

> "accuracy"

Measure:

### Quality

```text
field accuracy
document accuracy
missing fields
invalid fields
hallucinations
```

### Performance

```text
VLM latency
end-to-end latency
queue latency
```

### Reliability

```text
failure rate
retry rate
worker failures
```

### Economics

```text
cost/document
cost/field
```

This gives you an actual engineering dashboard.

---

# 15. Experiment Tracking

Instead of randomly changing prompts:

```text
v1
 ↓
change prompt
 ↓
v2
 ↓
change schema
 ↓
v3
```

Record:

```text
Experiment
Model
Prompt version
Dataset
Accuracy
Latency
Cost
Failure modes
```

Then you can say:

> "Experiment 4 improved field accuracy while increasing latency."

That's **AI engineering**, rather than prompt tinkering.

---

# 16. Human Review

Eventually:

```text
                    Extraction
                        │
                        ▼
                 Validation
                        │
               ┌────────┴────────┐
               │                 │
               ▼                 ▼
          High confidence    Low confidence
               │                 │
               ▼                 ▼
          Auto-approved      Human review
```

For Month 1, the review interface can be extremely simple.

Even an API response is enough.

We're establishing the **architecture**, not building a huge SaaS product.

---

# 17. Observability

Every request should be traceable.

Example:

```text
request_id
    ↓
document_id
    ↓
job_id
    ↓
worker
    ↓
VLM request
```

Then when someone says:

> "Document 839 failed."

you can trace:

```text
request
 → document
 → job
 → worker
 → VLM
 → validation
 → database
```

This is the mindset I want you developing.

---

# 18. Security

Even this first version should consider:

### Upload security

* file size limits
* MIME validation
* filename sanitization

### Authorization

```text
User A
   ↓
Document A

User B
   ↓
❌ Document A
```

### Secrets

No API keys in Git.

### Data

Documents may contain sensitive information, so think about:

* access control
* retention
* logging
* accidental data exposure

You don't need enterprise compliance in Month 1, but you should **think like the data matters**.

---

# 19. Deployment Architecture

Local:

```text
Docker Compose

Go
Python
Redis
Postgres
```

Production:

```text
                  Internet
                      │
                      ▼
                  Go API
                      │
             ┌────────┼────────┐
             ▼        ▼        ▼
         Postgres   Redis   Object Storage
                      │
                      ▼
                 Python Worker
                      │
                      ▼
                     VLM
```

The exact cloud provider is secondary.

The architecture is what matters.

---

# 🗓️ Month 1 Engineering Phases

Now the important part.

We're not really thinking:

> Week 1 = learn Go
> Week 2 = learn Python
> Week 3 = learn AI

That's too beginner-oriented.

Instead:

# Phase 1 — Build the Skeleton

### Week 1

**Theme: System boundaries**

Build:

```text
Go API
+
Postgres
+
Object storage
+
Redis
+
Worker
```

### Engineering concepts

* API design
* service boundaries
* context
* cancellation
* persistence
* asynchronous processing
* queues
* state machines

### Outcome

```text
Upload
 → stored
 → queued
 → worker receives job
```

---

# Phase 2 — Make AI Reliable

### Week 2

**Theme: AI inside a real system**

Build:

```text
Worker
 ↓
VLM
 ↓
Structured output
 ↓
Validation
 ↓
Persistence
```

### Engineering concepts

* VLM APIs
* structured generation
* schema validation
* retries
* backoff
* timeouts
* idempotency
* rate limiting
* failure handling

### Outcome

```text
Document
 ↓
AI
 ↓
validated structured data
```

---

# Phase 3 — Make AI Measurable

### Week 3

**Theme: Evaluation**

Build:

```text
Golden dataset
 ↓
Evaluation runner
 ↓
Metrics
 ↓
Experiments
 ↓
Regression detection
```

### Engineering concepts

* evaluation design
* ground truth
* test datasets
* metrics
* error taxonomy
* experimentation
* model comparison
* prompt/version management

### Outcome

You can answer:

> **"How good is this system?"**

with numbers.

---

# Phase 4 — Make It Production + Customer Real

### Week 4

**Theme: Deployment + ownership**

Build:

```text
Docker
 ↓
Production deployment
 ↓
Observability
 ↓
Failure testing
 ↓
Real user feedback
```

### Engineering concepts

* deployment
* containers
* production configuration
* logging
* observability
* load/failure testing
* security
* operational thinking
* customer discovery

### Outcome

You can answer:

> **"Can someone actually use this?"**

---

# 🧠 What You Should Be Able to Do After Month 1

This is the real curriculum.

You should be able to explain:

### Backend

* How do you design an API?
* How do you handle cancellation?
* How do you handle timeouts?
* How do you design job states?
* How do retries work?
* What is idempotency?
* What happens when a worker crashes?

### Distributed systems

* Why asynchronous processing?
* Why Redis?
* What happens when Redis fails?
* What happens when a worker dies?
* How do you prevent duplicate processing?
* How would you scale workers?

### AI engineering

* Why this VLM?
* How do you force structured output?
* How do you validate model output?
* What happens when the model is wrong?
* How do you handle low confidence?
* How do you compare models?

### Evaluation

* What is your ground truth?
* What does "accuracy" mean?
* Which fields fail most?
* How do you prevent regressions?
* What changed between V1 and V2?
* What does it cost per document?

### Production

* How do you deploy it?
* How do you observe it?
* How do you debug a failed document?
* What happens under load?
* What are the security risks?

### FDE

* What problem are you solving?
* Who actually experiences it?
* What does their current workflow look like?
* What would make them trust the system?
* Where should AI be used?
* Where should deterministic software be used?

---

# 🔥 The Skill Transformation

This is what I want Month 1 to accomplish:

```text
                BEFORE
                  │
                  ▼
        Full-Stack Engineer
                  │
        React + API + DB
                  │
                  ▼
              MONTH 1
                  │
      ┌───────────┼───────────┐
      ▼           ▼           ▼
 Distributed    AI          Evaluation
 Systems       Systems       Systems
      │           │           │
      └───────────┼───────────┘
                  ▼
          Production AI
             Engineer
                  │
                  ▼
        End-to-End Ownership
```

You're not abandoning full-stack.

You're **stacking capabilities on top of it**.

And that's important for the six-month goal: by the end of this month, you should still be capable of building the frontend, but you're no longer defining yourself by "I can connect frontend and backend."

You're demonstrating:

> **I can own an AI-powered production system from user/workflow → architecture → implementation → evaluation → deployment → operation.**

That is the foundation we'll build the remaining five months on.
