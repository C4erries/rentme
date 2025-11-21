import pandas as pd
from sklearn.compose import ColumnTransformer
from sklearn.impute import SimpleImputer
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score
from sklearn.model_selection import GridSearchCV, train_test_split
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import OneHotEncoder
from xgboost import XGBRegressor

from data_loader import (
    CATEGORICAL_FEATURES,
    FEATURE_COLUMNS,
    NUMERIC_FEATURES,
    TARGET_COLUMN,
    load_clean_test_data,
    load_clean_train_data,
    load_test_data,
    load_train_data,
)


def build_model_pipeline() -> Pipeline:
    numeric_pipeline = Pipeline(
        [
            ("imputer", SimpleImputer(strategy="median")),
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

    regressor = XGBRegressor(
        n_estimators=400,
        max_depth=6,
        learning_rate=0.05,
        subsample=0.8,
        colsample_bytree=0.8,
        objective="reg:squarederror",
        reg_lambda=1.0,
        random_state=42,
        n_jobs=-1,
    )

    pipeline = Pipeline(
        [
            ("preprocessor", preprocessor),
            ("regressor", regressor),
        ],
    )
    return pipeline


def tune_hyperparameters(train_df: pd.DataFrame) -> Pipeline:
    X = train_df[FEATURE_COLUMNS]
    y = train_df[TARGET_COLUMN]

    pipeline = build_model_pipeline()
    param_grid = {
        "regressor__n_estimators": [200, 400],
        "regressor__max_depth": [4, 6],
        "regressor__learning_rate": [0.05, 0.1],
        "regressor__subsample": [0.8],
        "regressor__colsample_bytree": [0.7, 0.9],
        "regressor__reg_lambda": [0.5, 1.0],
    }

    search = GridSearchCV(
        estimator=pipeline,
        param_grid=param_grid,
        scoring="neg_mean_absolute_error",
        cv=3,
        n_jobs=-1,
        verbose=1,
    )
    search.fit(X, y)
    print("\nBest hyperparameters:", search.best_params_)
    print("Best CV MAE:", -search.best_score_)
    return search.best_estimator_


def log_metrics(name: str, y_true: pd.Series, y_pred: pd.Series) -> None:
    print(f"\n{name} metrics:")
    print(f"  MSE: {mean_squared_error(y_true, y_pred):,.0f}")
    print(f"  MAE: {mean_absolute_error(y_true, y_pred):,.0f}")
    print(f"  R2 : {r2_score(y_true, y_pred):.3f}")


def train_model(train_df: pd.DataFrame, *, label: str) -> Pipeline:
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
    log_metrics(f"{label} validation", y_valid, y_pred)
    return pipeline


def evaluate_on_holdout(
    model: Pipeline,
    holdout_df: pd.DataFrame,
    *,
    label: str,
) -> pd.DataFrame:
    X_holdout = holdout_df[FEATURE_COLUMNS]
    y_holdout = holdout_df[TARGET_COLUMN]

    y_pred = model.predict(X_holdout)

    log_metrics(f"{label} holdout", y_holdout, y_pred)
    comparison = pd.DataFrame(
        {
            "price_real": y_holdout.reset_index(drop=True),
            "price_pred": pd.Series(y_pred),
        },
    )
    comparison["delta"] = comparison["price_pred"] - comparison["price_real"]

    print(f"\n{label} holdout comparison (first 20 rows):")
    print(comparison.head(20).to_string(index=False))
    return comparison


def report_feature_importances(model: Pipeline, *, label: str, top_n: int = 20) -> None:
    regressor = model.named_steps.get("regressor")
    preprocessor = model.named_steps.get("preprocessor")
    if regressor is None or preprocessor is None:
        print(f"\nNo feature importance available for {label} (missing steps).")
        return
    if not hasattr(regressor, "feature_importances_"):
        print(f"\nRegressor for {label} does not expose feature_importances_.")
        return
    feature_names = preprocessor.get_feature_names_out()
    importances = regressor.feature_importances_
    ranking = (
        pd.DataFrame({"feature": feature_names, "importance": importances})
        .sort_values("importance", ascending=False)
        .head(top_n)
    )
    print(f"\nTop {top_n} features for {label}:")
    print(ranking.to_string(index=False))


def main() -> None:
    raw_train_df = load_train_data()
    raw_test_df = load_test_data()
    print("\n=== Raw dataset ===")
    raw_model = train_model(raw_train_df, label="raw")
    evaluate_on_holdout(raw_model, raw_test_df, label="raw")

    clean_train_df = load_clean_train_data()
    clean_test_df = load_clean_test_data()
    print("\n=== Cleaned dataset ===")
    clean_model = train_model(clean_train_df, label="clean")
    evaluate_on_holdout(clean_model, clean_test_df, label="clean")
    report_feature_importances(clean_model, label="clean baseline")

    print("\n=== Tuned cleaned dataset (GridSearchCV) ===")
    tuned_model = tune_hyperparameters(clean_train_df)
    evaluate_on_holdout(tuned_model, clean_test_df, label="tuned clean")
    report_feature_importances(tuned_model, label="tuned clean")


if __name__ == "__main__":
    main()
