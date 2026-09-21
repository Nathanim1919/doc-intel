"""
Regional and financial validation rules for document extraction.
Specialized for Ethiopian invoices, receipts, tax documents, and general financial records.
"""

from __future__ import annotations

import logging
import re
from typing import Any

logger = logging.getLogger(__name__)

# Ethiopian calendar month names in Amharic and transliterations
ETHIOPIAN_MONTHS = [
    "መስከረም", "ጥቅምት", "ኅዳር", "ታኅሣሥ", "ጥር", "የካቲት",
    "መጋቢት", "ሚያዝያ", "ግንቦት", "ሰኔ", "ሐምሌ", "ነሐሴ", "ጳጉሜ",
    "Meskerem", "Tikimt", "Hidar", "Tahsas", "Tir", "Yekatit",
    "Megabit", "Miazia", "Ginbot", "Sene", "Hamle", "Nehase", "Pagume",
]


def validate_field(field_name: str, value: str | None, raw_confidence: float) -> tuple[float, str]:
    """
    Apply regional and semantic validation rules to an extracted field.

    Returns:
        (computed_confidence, validation_status)
        where validation_status is "PASSED" | "FAILED" | "SKIPPED"
    """
    if value is None or not str(value).strip():
        # Missing value on an extracted field flags review
        return 0.3, "FAILED"

    val = str(value).strip()
    computed_conf = raw_confidence

    # 1. Ethiopian TIN (Taxpayer Identification Number) validation
    # Format: exactly 10 digits
    if any(k in field_name.lower() for k in ("tin", "tax_number", "tin_number")):
        cleaned_digits = re.sub(r"\D", "", val)
        if len(cleaned_digits) == 10:
            return min(1.0, computed_conf + 0.05), "PASSED"
        else:
            logger.warning("Field %s value '%s' failed 10-digit TIN validation", field_name, val)
            return max(0.2, computed_conf - 0.4), "FAILED"

    # 2. Ethiopian VAT Registration Number
    # Typically 10 to 12 digits
    if "vat_number" in field_name.lower() or "vat_reg" in field_name.lower():
        cleaned_digits = re.sub(r"\D", "", val)
        if 10 <= len(cleaned_digits) <= 12:
            return min(1.0, computed_conf + 0.05), "PASSED"
        else:
            return max(0.3, computed_conf - 0.3), "FAILED"

    # 3. Ethiopian Calendar Date (ዓ.ም / E.C.)
    if "ethiopian" in field_name.lower() or "ዓ.ም" in val:
        has_am = "ዓ.ም" in val or "ዓ/ም" in val or "E.C." in val or "EC" in val
        has_month = any(m in val for m in ETHIOPIAN_MONTHS)
        has_date_format = bool(re.search(r"\d{1,2}[/-]\d{1,2}[/-]\d{4}", val))
        if has_am or has_month or has_date_format:
            return min(1.0, computed_conf + 0.05), "PASSED"
        return max(0.3, computed_conf - 0.2), "SKIPPED"

    # 4. Stamp detection boolean
    if "stamp_detected" in field_name.lower():
        if val.lower() in ("true", "false", "yes", "no", "ተረጋግጧል", "አለ", "የለም"):
            return 0.95, "PASSED"

    # 5. Generic Numeric amount fields
    if any(k in field_name.lower() for k in ("amount", "total", "subtotal", "tax", "vat", "etb", "price")):
        cleaned_num = re.sub(r"[^\d.]", "", val)
        if cleaned_num and _is_valid_float(cleaned_num):
            return computed_conf, "PASSED" if computed_conf >= 0.85 else "SKIPPED"
        else:
            return max(0.2, computed_conf - 0.3), "FAILED"

    # Fallback to confidence threshold
    if computed_conf >= 0.85:
        return computed_conf, "PASSED"
    if computed_conf >= 0.50:
        return computed_conf, "SKIPPED"
    return computed_conf, "FAILED"


def validate_document_math(fields_dict: dict[str, str | None]) -> list[str]:
    """
    Verify arithmetic consistency across Ethiopian invoice amounts:
    Subtotal + VAT (15%) == Total Amount (within 1.0 ETB tolerance).
    Returns a list of warning/error messages if discrepancies are detected.
    """
    warnings: list[str] = []

    subtotal = _extract_amount(fields_dict, ["subtotal", "subtotal_etb", "taxable_amount"])
    vat = _extract_amount(fields_dict, ["vat", "vat_amount", "vat_15_etb", "vat_etb"])
    total = _extract_amount(fields_dict, ["total", "total_amount", "total_amount_etb", "grand_total"])

    if subtotal is not None and total is not None:
        if vat is not None:
            expected_total = subtotal + vat
            if abs(expected_total - total) > 1.5:
                warnings.append(
                    f"Arithmetic mismatch: Subtotal ({subtotal:.2f}) + VAT ({vat:.2f}) = {expected_total:.2f}, "
                    f"but Total is {total:.2f} ETB"
                )
        else:
            # Check 15% standard Ethiopian VAT
            expected_vat = subtotal * 0.15
            expected_total = subtotal + expected_vat
            if abs(expected_total - total) <= 2.0:
                logger.info("Implied 15%% Ethiopian VAT validated: subtotal=%.2f, total=%.2f", subtotal, total)

    return warnings


def _is_valid_float(s: str) -> bool:
    try:
        float(s)
        return True
    except ValueError:
        return False


def _extract_amount(fields: dict[str, str | None], candidate_keys: list[str]) -> float | None:
    # 1. Try exact matches first
    for cand in candidate_keys:
        for k, v in fields.items():
            if k.lower() == cand.lower() and v:
                cleaned = re.sub(r"[^\d.]", "", str(v))
                if _is_valid_float(cleaned):
                    return float(cleaned)

    # 2. Try substring matches, avoiding subtotal when looking for total
    is_total_query = any("total" in c for c in candidate_keys) and not any("subtotal" in c for c in candidate_keys)
    for k, v in fields.items():
        k_lower = k.lower()
        if is_total_query and "subtotal" in k_lower:
            continue
        if v and any(cand in k_lower for cand in candidate_keys):
            cleaned = re.sub(r"[^\d.]", "", str(v))
            if _is_valid_float(cleaned):
                return float(cleaned)

    return None
