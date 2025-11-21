"""Lightweight data quality report for Rentme datasets."""

from typing import Iterable

import pandas as pd

from data_loader import (
    CATEGORICAL_FEATURES,
    FEATURE_COLUMNS,
    NUMERIC_FEATURES,
    TARGET_COLUMN,
    load_test_data,
    load_train_data,
)


def _section(title: str) -> None:
    print(f"\n{'=' * 10} {title} {'=' * 10}")


def report_high_level(df: pd.DataFrame, label: str) -> None:
    _section(f"{label} overview")
    print(f"shape: {df.shape}")
    df[FEATURE_COLUMNS + [TARGET_COLUMN]].info(verbose=False)
    missing = df[FEATURE_COLUMNS].isna().sum()
    missing = missing[missing > 0].sort_values(ascending=False)
    if not missing.empty:
        print("\nmissing by column:")
        print(missing)
    else:
        print("\nno missing values in feature columns")


def report_numeric(df: pd.DataFrame, columns: Iterable[str]) -> None:
    _section("numeric summary")
    print(df[columns].describe(include="number").T)
    corr = df[columns].corr()
    print("\npairwise Pearson correlation (top-left block):")
    print(corr.iloc[:5, :5])


def report_categorical(df: pd.DataFrame, columns: Iterable[str]) -> None:
    _section("categorical summary")
    for column in columns:
        print(f"\n{column}:")
        top = df[column].value_counts(dropna=False).head(8)
        print(top.to_string())


def report_outliers(df: pd.DataFrame) -> None:
    _section("outlier checks")
    rooms_non_numeric = df[df["rooms"].apply(lambda value: not isinstance(value, (int, float)))]
    print("non-numeric rooms count:", len(rooms_non_numeric))
    if not rooms_non_numeric.empty:
        print(rooms_non_numeric[["metro", "rooms", "total_area"]].head(5))

    unrealistic_prices = df[df["price"] > df["price"].quantile(0.99)]
    print("top 5 price anomalies:")
    print(unrealistic_prices[["metro", "price", "total_area"]].sort_values("price", ascending=False).head(5))


def report_duplicates(df: pd.DataFrame) -> None:
    _section("duplicates")
    dupe_count = df.duplicated(subset=["metro", "price", "total_area", "rooms"]).sum()
    print("potential duplicates:", dupe_count)


def main() -> None:
    train = load_train_data()
    test = load_test_data()

    report_high_level(train, "TRAIN")
    report_numeric(train, NUMERIC_FEATURES + [TARGET_COLUMN])
    report_categorical(train, CATEGORICAL_FEATURES)
    report_outliers(train)
    report_duplicates(train)

    report_high_level(test, "TEST")
    report_numeric(test, NUMERIC_FEATURES + [TARGET_COLUMN])
    report_categorical(test, CATEGORICAL_FEATURES)


if __name__ == "__main__":
    main()
