"""
Gemini-based document extraction.

Flow:
  1. Receive raw file bytes + MIME type
  2. Base64-encode and send to Gemini (multimodal) with a structured-output schema
  3. Parse the response into a list of ExtractedField objects

The model is asked to return a flat key-value list so any document type
(invoice, passport, contract, etc.) is handled generically.

Model fallback:
  Models are tried in the order given by CANDIDATE_MODELS.  If one returns an
  API error (quota, rate-limit, unavailable) we log a warning and move to the
  next.  The model that actually succeeded is returned alongside the result so
  it can be stored in extraction_runs.model.
"""

import base64
import json
import logging
import time
from typing import Any

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Ranked list of free / low-cost Gemini models to try in order.
# The first one that succeeds wins; if all fail the last exception is raised.
# ---------------------------------------------------------------------------
CANDIDATE_MODELS: list[str] = [
    "gemini-3.5-flash-lite",   # fastest + cheapest — try first
    "gemini-3.5-flash",
    "gemini-3.6-flash",
    "gemini-flash-latest",     # always points at the latest flash release
]

from google import genai
from pydantic import BaseModel, Field


# ---------------------------------------------------------------------------
# Output schema — Pydantic model drives the JSON schema sent to Gemini
# ---------------------------------------------------------------------------

class ExtractedField(BaseModel):
    field_name: str = Field(description="Canonical snake_case name of the extracted field, e.g. invoice_number")
    value: str | None = Field(description="Extracted text value; null if the field is present but unreadable")
    confidence: float = Field(ge=0.0, le=1.0, description="Model's confidence in this extraction, 0.0–1.0")
    page_number: int | None = Field(description="1-based page number where the value was found; null if unknown")


class ExtractionResult(BaseModel):
    fields: list[ExtractedField] = Field(description="All key-value pairs extracted from the document")
    document_type: str | None = Field(description="Inferred document type, e.g. 'invoice', 'passport', 'contract'")


# ---------------------------------------------------------------------------
# Prompt
# ---------------------------------------------------------------------------

SYSTEM_INSTRUCTION = """You are a high-precision document extraction engine specializing in commercial invoices, fiscal receipts, and contracts, including bilingual English and Amharic (Ethiopian / Ge'ez script) documents.

Your job is to extract every meaningful structured field from the provided document image or PDF.

General Extraction Rules:
- Use canonical snake_case for field names (e.g. seller_name, invoice_date, total_amount).
- If a field value is partially visible or low-confidence, still extract it but set confidence accordingly.
- Never invent data. If a field does not exist in the document, omit it — do not include null values for missing fields.
- page_number is 1-based. Set to null only if genuinely unknown.
- Your response MUST be valid JSON matching the provided schema exactly.

Ethiopian & Amharic Document Rules:
1. Fiscal Identifiers:
   - Extract TIN (የግብር ከፋይ መለያ ቁጥር) as 'tin_number'. Ensure all 10 digits are preserved accurately.
   - Extract VAT registration number (የተጨማሪ እሴት ታክስ ምዝገባ ቁጥር) as 'vat_number'.
   - Extract Machine Registration Code (የማሽን ምዝገባ ቁጥር) as 'mrc_number'.
   - Extract Fiscal Receipt sequence number (ደረሰኝ ቁጥር / FS No.) as 'fs_number'.
2. Dates & Calendars:
   - For Ethiopian Calendar dates (containing 'ዓ.ም', 'ዓ/ም', or Amharic months: መስከረም, ጥቅምት, ኅዳር, ታኅሣሥ, ጥር, የካቲት, መጋቢት, ሚያዝያ, ግንቦት, ሰኔ, ሐምሌ, ነሐሴ, ጳጉሜ), extract the raw string into 'invoice_date_ethiopian'.
   - If a Gregorian date is also present or can be reliably inferred, extract it into 'invoice_date_gregorian' (YYYY-MM-DD).
3. Currency & Amounts (Ethiopian Birr / ETB / ብር):
   - Extract numeric amounts as clean numbers (e.g. "1500.00") without currency symbols or commas into:
     * 'subtotal_etb' (pre-tax amount / ከመቀነሱ በፊት)
     * 'vat_15_etb' (15% VAT amount / ተጨማሪ እሴት ታክስ 15%)
     * 'withholding_tax_etb' (if withholding tax is deducted)
     * 'total_amount_etb' (grand total / ጠቅላላ ድምር)
4. Official Stamps & Signatures:
   - Inspect the document for physical rubber ink stamps (ማኅተም). If present, output 'stamp_detected' as 'true' and extract readable text inside the stamp into 'stamp_organization'. If absent, output 'stamp_detected' as 'false'.
   - If an authorized signature is present, output 'signature_detected' as 'true'.
5. Script Preservation:
   - Accurately preserve Amharic/Ge'ez text for company names (e.g. 'ሻጭ' / 'ገዥ'), descriptions, and locations.
"""

USER_PROMPT = """Extract all structured key-value fields from this document. If this is an Ethiopian receipt or invoice, adhere to the fiscal identification, Ethiopian calendar date (ዓ.ም), ETB currency amounts, and stamp verification rules."""


# ---------------------------------------------------------------------------
# Extractor
# ---------------------------------------------------------------------------

class Extractor:
    """Wraps the Gemini Interactions API for document field extraction.

    Tries CANDIDATE_MODELS in order.  The ``model`` argument passed at
    construction is used as an *override* (e.g. from env var) when provided;
    otherwise the module-level CANDIDATE_MODELS list is used.
    """

    def __init__(self, api_key: str, model: str | None = None) -> None:
        # If an explicit model was configured, put it first in the list so it
        # is preferred but we still fall back if it fails.
        if model and model not in CANDIDATE_MODELS:
            self._models = [model] + CANDIDATE_MODELS
        elif model:
            # Reorder: put requested model first
            rest = [m for m in CANDIDATE_MODELS if m != model]
            self._models = [model] + rest
        else:
            self._models = CANDIDATE_MODELS

        self._client = genai.Client(api_key=api_key)
        logger.info("Extractor ready — model order: %s", self._models)

    def extract(
        self,
        file_bytes: bytes,
        mime_type: str,
    ) -> tuple[ExtractionResult, str, int, int]:
        """
        Run extraction on raw file bytes, trying each candidate model in order.

        Returns:
            (result, model_used, latency_ms, tokens_used)

        Raises:
            ValueError: if every model returns malformed JSON on its last attempt
            Exception: the last API exception if all models fail with API errors
        """
        b64data = base64.b64encode(file_bytes).decode("utf-8")
        last_exc: Exception | None = None

        for model in self._models:
            try:
                logger.info("Trying model: %s", model)
                t0 = time.monotonic()
                interaction = self._client.interactions.create(
                    model=model,
                    system_instruction=SYSTEM_INSTRUCTION,
                    input=[
                        {
                            "type": "image",
                            "mime_type": mime_type,
                            "data": b64data,
                        },
                        {"type": "text", "text": USER_PROMPT},
                    ],
                    response_format=[
                        {
                            "type": "text",
                            "mime_type": "application/json",
                            "schema": ExtractionResult.model_json_schema(),
                        }
                    ],
                    store=False,
                )
                latency_ms = int((time.monotonic() - t0) * 1000)
                raw_text = interaction.output_text or ""
                tokens_used = _safe_tokens(interaction)

                # May raise ValueError — propagate immediately (not a model
                # availability issue; retrying with another model won't help).
                result = _parse_result(raw_text)
                logger.info("Extraction succeeded — model=%s latency=%dms tokens=%d",
                            model, latency_ms, tokens_used)
                return result, model, latency_ms, tokens_used

            except ValueError:
                # Malformed JSON from this specific model — re-raise, no point
                # trying others since they'd get the same bad output.
                raise
            except Exception as exc:  # noqa: BLE001
                logger.warning("Model %s failed (%s: %s) — trying next",
                               model, type(exc).__name__, exc)
                last_exc = exc
                continue

        # All models exhausted
        raise last_exc or RuntimeError("All candidate models failed")


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _safe_tokens(interaction: Any) -> int:
    """Extract total_tokens from interaction usage, defaulting to 0."""
    try:
        return interaction.usage.total_tokens or 0
    except AttributeError:
        return 0


def _parse_result(raw_text: str) -> ExtractionResult:
    """
    Parse the model's JSON output into an ExtractionResult.
    Raises ValueError with the raw text on parse failure so the caller
    can store the error and mark the job FAILED.
    """
    try:
        data = json.loads(raw_text)
        return ExtractionResult.model_validate(data)
    except (json.JSONDecodeError, ValueError) as exc:
        raise ValueError(f"Gemini returned unparseable output: {exc!s}\nRaw: {raw_text[:500]}") from exc
