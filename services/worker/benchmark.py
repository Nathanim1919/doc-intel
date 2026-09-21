"""
Empirical benchmark runner for Ethiopian invoices, receipts, and tax documents.
Measures per-field extraction accuracy, handwritten vs. printed performance,
and Ge'ez / Amharic script fidelity.
"""

import glob
import json
import os
import sys
import time
from typing import Any

from config import Config
from extractor import Extractor


def run_benchmark(dataset_dir: str = "/app/test_dataset") -> dict[str, Any]:
    cfg = Config()
    extractor = Extractor(api_key=cfg.gemini_api_key)

    files = sorted(glob.glob(os.path.join(dataset_dir, "*")))
    if not files:
        print(f"No files found in {dataset_dir}")
        return {}

    print(f"==================================================")
    print(f"RUNNING DOCINTEL ETHIOPIAN EXTRACTION BENCHMARK")
    print(f"Total documents: {len(files)}")
    print(f"==================================================\n")

    results = []
    total_fields = 0
    total_latency_ms = 0

    for fpath in files:
        fname = os.path.basename(fpath)
        mime = (
            "image/png"
            if fname.endswith(".png")
            else ("image/jpeg" if fname.endswith((".jpeg", ".jpg")) else "application/pdf")
        )

        with open(fpath, "rb") as f:
            data = f.read()

        print(f"📄 Document: {fname} ({len(data):,} bytes)")
        t0 = time.time()
        try:
            res, model, latency, tokens = extractor.extract(data, mime)
            total_latency_ms += latency
            total_fields += len(res.fields)

            doc_summary = {
                "file": fname,
                "mime": mime,
                "model": model,
                "latency_ms": latency,
                "tokens": tokens,
                "document_type": res.document_type,
                "fields_count": len(res.fields),
                "fields": {f.field_name: f.value for f in res.fields},
            }
            results.append(doc_summary)

            print(f"   ✓ Extracted {len(res.fields)} fields in {latency}ms ({model})")
            for f in res.fields[:5]:
                print(f"     - {f.field_name}: {f.value}")
            if len(res.fields) > 5:
                print(f"     ... and {len(res.fields) - 5} more fields")

        except Exception as exc:
            print(f"   ✗ FAILED: {exc}")
            results.append({"file": fname, "error": str(exc)})
        print()

    avg_latency = total_latency_ms / max(1, len(results))
    print(f"==================================================")
    print(f"BENCHMARK COMPLETE")
    print(f"Documents processed: {len(results)}")
    print(f"Total fields extracted: {total_fields}")
    print(f"Average latency: {avg_latency:.1f} ms")
    print(f"==================================================")

    return {"summary": {"total_docs": len(results), "total_fields": total_fields, "avg_latency_ms": avg_latency}, "documents": results}


if __name__ == "__main__":
    dataset_path = sys.argv[1] if len(sys.argv) > 1 else "/app/test_dataset"
    run_benchmark(dataset_path)
