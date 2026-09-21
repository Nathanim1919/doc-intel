"""Worker configuration — loaded once at startup from environment variables."""

import os
import sys


def _require(key: str) -> str:
    val = os.getenv(key, "").strip()
    if not val:
        print(f"[config] FATAL: required env var {key!r} is missing or empty", flush=True)
        sys.exit(1)
    return val


class Config:
    # Redis
    redis_url: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")
    queue_key: str = os.getenv("QUEUE_KEY", "doc_intel:jobs")
    brpop_timeout: int = int(os.getenv("BRPOP_TIMEOUT_SECONDS", "5"))

    # Postgres
    postgres_dsn: str

    # MinIO / S3-compatible object storage
    minio_endpoint: str
    minio_access_key: str
    minio_secret_key: str
    minio_bucket: str
    minio_use_ssl: bool

    # Gemini
    gemini_api_key: str
    gemini_model: str = os.getenv("GEMINI_MODEL", "gemini-3.8-flash")
    prompt_version: str = os.getenv("PROMPT_VERSION", "v1")

    # Retry
    max_attempts: int = int(os.getenv("MAX_JOB_ATTEMPTS", "3"))

    def __init__(self) -> None:
        self.postgres_dsn = _require("POSTGRES_DSN")
        self.minio_endpoint = _require("MINIO_ENDPOINT")
        self.minio_access_key = _require("MINIO_ACCESS_KEY")
        self.minio_secret_key = _require("MINIO_SECRET_KEY")
        self.minio_bucket = os.getenv("MINIO_BUCKET", "doc-intel")
        self.minio_use_ssl = os.getenv("MINIO_USE_SSL", "false").lower() == "true"
        self.gemini_api_key = _require("GEMINI_API_KEY")
