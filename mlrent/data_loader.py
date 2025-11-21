from __future__ import annotations

from pathlib import Path
from typing import Iterable

import pandas as pd

DATA_DIRECTORY = Path(__file__).resolve().parent / "data"

NUMERIC_FEATURES = [
    "minutes",
    "fee_percent",
    "views",
    "storey",
    "storeys",
    "rooms",
    "total_area",
    "living_area",
    "kitchen_area",
]
TARGET_COLUMN = "price"
CATEGORICAL_FEATURES = ["metro", "way", "provider"]
FEATURE_COLUMNS = NUMERIC_FEATURES + CATEGORICAL_FEATURES

MAX_STOREY = 80
MAX_STOREYS = 80
PRICE_OUTLIER_QUANTILE = 0.995
AREA_RULES = {
    "total_area": {"min": 10, "quantile": 0.995},
    "living_area": {"min": 8, "quantile": 0.995},
    "kitchen_area": {"min": 4, "quantile": 0.995},
}
DUPLICATE_KEYS = ["metro", "price", "total_area", "rooms"]
PROVIDER_ALIASES = {
    "agency": "agency",
    "агентство": "agency",
    "developer": "developer",
    "застройщик": "developer",
    "застройщик ": "developer",
    "owner": "owner",
    "владелец": "owner",
    "realtor": "realtor",
}


def _strip_strings(df: pd.DataFrame, columns: Iterable[str]) -> None:
    for column in columns:
        if column in df:
            df[column] = (
                df[column]
                .fillna("unknown")
                .astype(str)
                .str.strip()
            )
            df.loc[df[column] == "", column] = "unknown"


def _coerce_numeric(df: pd.DataFrame, columns: Iterable[str]) -> None:
    for column in columns:
        if column in df:
            df[column] = pd.to_numeric(df[column], errors="coerce")


def _load_raw_dataframe(filename: str) -> pd.DataFrame:
    path = DATA_DIRECTORY / filename
    df = pd.read_csv(path, index_col=0)
    df = df.rename(columns=lambda col: col.strip())
    df = df.copy()
    _strip_strings(df, CATEGORICAL_FEATURES)
    _coerce_numeric(df, NUMERIC_FEATURES + [TARGET_COLUMN])
    return df.dropna(subset=[TARGET_COLUMN]).reset_index(drop=True)


def _normalize_provider(df: pd.DataFrame) -> None:
    if "provider" not in df:
        return
    normalized = (
        df["provider"]
        .fillna("unknown")
        .astype(str)
        .str.strip()
        .str.lower()
    )
    df["provider"] = normalized.map(PROVIDER_ALIASES).fillna(normalized)


def _remove_area_outliers(df: pd.DataFrame) -> pd.DataFrame:
    cleaned = df
    for column, rules in AREA_RULES.items():
        if column not in cleaned:
            continue
        min_value = rules.get("min")
        quantile = rules.get("quantile")
        mask = pd.Series(True, index=cleaned.index)
        if min_value is not None:
            mask &= cleaned[column].isna() | (cleaned[column] >= min_value)
        if quantile is not None:
            upper = cleaned[column].quantile(quantile)
            mask &= cleaned[column].isna() | (cleaned[column] <= upper)
        cleaned = cleaned[mask]
    return cleaned


def clean_dataframe(df: pd.DataFrame) -> pd.DataFrame:
    cleaned = df.copy()
    _normalize_provider(cleaned)

    # Floor sanity checks.
    cleaned.loc[(cleaned["storey"] <= 0) | (cleaned["storey"] > MAX_STOREY), "storey"] = pd.NA
    cleaned.loc[(cleaned["storeys"] <= 0) | (cleaned["storeys"] > MAX_STOREYS), "storeys"] = pd.NA
    cleaned.loc[
        cleaned["storey"].notna()
        & cleaned["storeys"].notna()
        & (cleaned["storey"] > cleaned["storeys"]),
        "storey",
    ] = pd.NA

    # Rooms sanity.
    cleaned.loc[(cleaned["rooms"] <= 0) | (cleaned["rooms"] > 10), "rooms"] = pd.NA

    # Outlier removal.
    upper_price = cleaned[TARGET_COLUMN].quantile(PRICE_OUTLIER_QUANTILE)
    cleaned = cleaned[cleaned[TARGET_COLUMN] <= upper_price]
    cleaned = _remove_area_outliers(cleaned)
    cleaned = cleaned.drop_duplicates(subset=DUPLICATE_KEYS, keep="first")

    # Imputation.
    for column in ["rooms", "storey", "storeys"]:
        if column not in cleaned:
            continue
        median_value = cleaned[column].median()
        if pd.isna(median_value):
            median_value = 0
        cleaned[column] = cleaned[column].fillna(median_value)
    cleaned["rooms"] = cleaned["rooms"].round().astype(int)

    return cleaned.reset_index(drop=True)


def load_train_data() -> pd.DataFrame:
    return _load_raw_dataframe("data.csv")


def load_test_data() -> pd.DataFrame:
    return _load_raw_dataframe("test.csv")


def load_clean_train_data() -> pd.DataFrame:
    return clean_dataframe(load_train_data())


def load_clean_test_data() -> pd.DataFrame:
    return clean_dataframe(load_test_data())
