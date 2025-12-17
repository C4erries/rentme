from typing import Literal, Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from mlrent.ml import (
    build_feature_vector_from_dict,
    dataset_paths_for_term,
    evaluate_model,
    parser,
    predict,
    train_long_term,
    train_short_term,
)

app = FastAPI()

MODEL = None  # legacy alias for long-term model
LONG_MODEL = None
SHORT_MODEL = None


class PredictRequest(BaseModel):
    rental_term: Optional[Literal["short_term", "long_term"]] = None
    city: str = "Moscow"
    minutes: float
    way: str  # "walk" or "car"
    rooms: int
    total_area: float
    storey: float
    storeys: float
    renovation: int
    building_age_years: int
    listing_id: Optional[str] = None
    current_price: Optional[float] = None


class PredictResponse(BaseModel):
    listing_id: Optional[str] = None
    rental_term: str
    recommended_price: float
    current_price: Optional[float] = None
    diff: Optional[float] = None


class ModelMetrics(BaseModel):
    mae: float
    rmse: float
    train_size: int
    test_size: int


class MetricsResponse(BaseModel):
    short_term: ModelMetrics
    long_term: ModelMetrics


@app.on_event("startup")
async def startup_event():
    global MODEL, LONG_MODEL, SHORT_MODEL
    LONG_MODEL = train_long_term()
    SHORT_MODEL = train_short_term()
    MODEL = LONG_MODEL


@app.get("/health")
async def health():
    return {"status": "ok"}


def _select_model(term: str):
    if term == "short_term":
        return SHORT_MODEL
    return LONG_MODEL


def _predict_with_term(request: PredictRequest, rental_term: str) -> PredictResponse:
    model = _select_model(rental_term)
    if model is None:
        raise HTTPException(status_code=503, detail="Model is not loaded")

    try:
        feature_vector = build_feature_vector_from_dict(request.dict())
        recommended_price = predict(feature_vector, model)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    diff = None
    if request.current_price is not None:
        diff = request.current_price - recommended_price

    return PredictResponse(
        listing_id=request.listing_id,
        rental_term=rental_term,
        recommended_price=recommended_price,
        current_price=request.current_price,
        diff=diff,
    )


@app.post("/predict", response_model=PredictResponse)
async def predict_price(request: PredictRequest):
    rental_term = request.rental_term or "long_term"
    if rental_term not in {"short_term", "long_term"}:
        raise HTTPException(status_code=400, detail="rental_term must be short_term or long_term")
    return _predict_with_term(request, rental_term)


@app.post("/predict/short", response_model=PredictResponse)
async def predict_short(request: PredictRequest):
    request.rental_term = "short_term"
    return _predict_with_term(request, "short_term")


@app.post("/predict/long", response_model=PredictResponse)
async def predict_long(request: PredictRequest):
    request.rental_term = "long_term"
    return _predict_with_term(request, "long_term")


@app.get("/metrics", response_model=MetricsResponse)
async def metrics():
    if SHORT_MODEL is None or LONG_MODEL is None:
        raise HTTPException(status_code=503, detail="Models are not loaded")
    return MetricsResponse(
        short_term=_metrics_for_term("short_term", SHORT_MODEL),
        long_term=_metrics_for_term("long_term", LONG_MODEL),
    )


def _metrics_for_term(term: str, model):
    train_path, test_path = dataset_paths_for_term(term)
    train_data, train_labels = parser(str(train_path))
    test_data, test_labels = parser(str(test_path))
    mae, rmse = evaluate_model(model, test_data, test_labels)
    return ModelMetrics(
        mae=mae,
        rmse=rmse,
        train_size=len(train_labels),
        test_size=len(test_labels),
    )


if __name__ == "__main__":
    import uvicorn

    uvicorn.run("mlrent.main:app", host="0.0.0.0", port=8000, reload=True)
