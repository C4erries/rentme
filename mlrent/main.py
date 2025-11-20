import pandas as pd
from sklearn.compose import ColumnTransformer
from sklearn.impute import SimpleImputer
from sklearn.linear_model import LinearRegression
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score
from sklearn.model_selection import train_test_split
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import OneHotEncoder, StandardScaler

from data_loader import (
    CATEGORICAL_FEATURES,
    FEATURE_COLUMNS,
    NUMERIC_FEATURES,
    TARGET_COLUMN,
    load_test_data,
    load_train_data,
)


def build_model_pipeline() -> Pipeline:
    numeric_pipeline = Pipeline(
        [
            ("imputer", SimpleImputer(strategy="median")),
            ("scaler", StandardScaler()),
        ],
    )
    categorical_pipeline = Pipeline(
        [
            ("imputer", SimpleImputer(strategy="most_frequent")),
            ("onehot", OneHotEncoder(handle_unknown="ignore")),
        ],
    )
    preprocessor = ColumnTransformer(
        [
            ("num", numeric_pipeline, NUMERIC_FEATURES),
            ("cat", categorical_pipeline, CATEGORICAL_FEATURES),
        ],
        remainder="drop",
    )

    pipeline = Pipeline(
        [
            ("preprocessor", preprocessor),
            ("regressor", LinearRegression()),
        ],
    )
    return pipeline


def log_metrics(name: str, y_true: pd.Series, y_pred: pd.Series) -> None:
    print(f"\n{name} metrics:")
    print(f"  MSE: {mean_squared_error(y_true, y_pred):,.0f}")
    print(f"  MAE: {mean_absolute_error(y_true, y_pred):,.0f}")
    print(f"  R2 : {r2_score(y_true, y_pred):.3f}")


def train_model(train_df: pd.DataFrame) -> Pipeline:
    pipeline = build_model_pipeline()
    X = train_df[FEATURE_COLUMNS]
    y = train_df[TARGET_COLUMN]

    X_train, X_valid, y_train, y_valid = train_test_split(
        X,
        y,
        test_size=0.2,
        random_state=42,
    )

    pipeline.fit(X_train, y_train)
    y_pred = pipeline.predict(X_valid)
    log_metrics("validation", y_valid, y_pred)
    return pipeline


def evaluate_on_holdout(model: Pipeline, holdout_df: pd.DataFrame) -> pd.DataFrame:
    X_holdout = holdout_df[FEATURE_COLUMNS]
    y_holdout = holdout_df[TARGET_COLUMN]

    y_pred = model.predict(X_holdout)

    log_metrics("holdout", y_holdout, y_pred)
    comparison = pd.DataFrame(
        {
            "price_real": y_holdout.reset_index(drop=True),
            "price_pred": pd.Series(y_pred),
        },
    )
    comparison["delta"] = comparison["price_pred"] - comparison["price_real"]

    print("\nHoldout comparison (first 20 rows):")
    print(comparison.head(20).to_string(index=False))
    return comparison


def main() -> None:
    train_df = load_train_data()
    test_df = load_test_data()

    model = train_model(train_df)
    evaluate_on_holdout(model, test_df)


if __name__ == "__main__":
    main()
