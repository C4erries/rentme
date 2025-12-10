from pathlib import Path
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from mlrent.ml import build_feature_vector_from_dict, predict, train_from_csv


app = FastAPI()

MODEL = None


class PredictRequest(BaseModel):
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
    recommended_price: float
    current_price: Optional[float] = None
    diff: Optional[float] = None


@app.on_event("startup")
async def startup_event():
    global MODEL
    train_path = Path(__file__).with_name("clean_train.csv")
    MODEL = train_from_csv(str(train_path))


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.post("/predict", response_model=PredictResponse)
async def predict_price(request: PredictRequest):
    if MODEL is None:
        raise HTTPException(status_code=503, detail="Model is not loaded")
    try:
        feature_vector = build_feature_vector_from_dict(request.dict())
        recommended_price = predict(feature_vector, MODEL)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    diff = None
    if request.current_price is not None:
        diff = request.current_price - recommended_price

    return PredictResponse(
        listing_id=request.listing_id,
        recommended_price=recommended_price,
        current_price=request.current_price,
        diff=diff,
    )


if __name__ == "__main__":
    import uvicorn

    uvicorn.run("mlrent.main:app", host="0.0.0.0", port=8000, reload=True)
