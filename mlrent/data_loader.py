from pathlib import Path

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


def _strip_strings(df: pd.DataFrame, columns: list[str]) -> None:
    for column in columns:
        if column in df:
            df[column] = (
                df[column]
                .fillna("unknown")
                .astype(str)
                .str.strip()
            )
            df.loc[df[column] == "", column] = "unknown"


def _coerce_numeric(df: pd.DataFrame, columns: list[str]) -> None:
    for column in columns:
        if column in df:
            df[column] = pd.to_numeric(df[column], errors="coerce")


def load_dataframe(filename: str) -> pd.DataFrame:
    path = DATA_DIRECTORY / filename
    df = pd.read_csv(path, index_col=0)
    df = df.rename(columns=lambda col: col.strip())
    df = df.copy()
    _strip_strings(df, CATEGORICAL_FEATURES)
    _coerce_numeric(df, NUMERIC_FEATURES + [TARGET_COLUMN])
    return df.dropna(subset=[TARGET_COLUMN]).reset_index(drop=True)


def load_train_data() -> pd.DataFrame:
    return load_dataframe("data.csv")


def load_test_data() -> pd.DataFrame:
    return load_dataframe("test.csv")
