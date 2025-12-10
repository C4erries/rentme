import csv
from pathlib import Path

import numpy as np
from sklearn.linear_model import Lasso
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import PolynomialFeatures, StandardScaler


# В учебном сервисе работаем в исходных единицах цены,
# поэтому масштабирование через константу пока не используем.
PRICE_SCALE = 1.0


def parser(file: str):
    """
    Читает CSV и возвращает матрицу признаков и вектор целевой переменной.

    Ожидаем схему:
    id,city,price,minutes,way,rooms,total_area,storey,storeys,renovation,building_age_years
    """
    rows = []
    with open(file, "r", encoding="utf-8", newline="") as f:
        reader = csv.DictReader(f)
        for row in reader:
            rows.append(row)

    if not rows:
        return np.empty((0, 8), dtype=float), np.empty(0, dtype=float)

    way_map = {"walk": 0.0, "car": 1.0}
    data = []
    labels = []
    for row in rows:
        label = float(row["price"])  # работаем в исходных единицах
        way = way_map.get(row["way"].strip().lower(), 0.0)
        features = [
            float(row["minutes"]),
            float(way),
            float(row["rooms"]),
            float(row["total_area"]),
            float(row["storey"]),
            float(row["storeys"]),
            float(row["renovation"]),
            float(row["building_age_years"]),
        ]
        data.append(features)
        labels.append(label)

    data = np.array(data, dtype=float)
    labels = np.array(labels, dtype=float)
    return data, labels


def build_model() -> Pipeline:
    """
    Строит модель, аналогичную той, что предложил коллега,
    но для новой схемы признаков.
    """
    model = Pipeline(
        [
            ("poly", PolynomialFeatures(degree=10)),
            ("scaler", StandardScaler()),
            ("lasso", Lasso(alpha=10000, max_iter=10000)),
        ]
    )
    return model


def predict(features, model: Pipeline) -> float:
    """
    Делает предсказание по списку числовых признаков и обученной модели.
    """
    features = np.array(features, dtype=float).reshape(1, -1)
    return float(model.predict(features)[0])


def build_feature_vector_from_dict(data):
    way_map = {'walk': 0.0, 'car': 1.0}
    way_raw = str(data['way']).strip().lower()
    if way_raw not in way_map:
        raise ValueError('way must be "walk" or "car"')
    return [
        float(data['minutes']),
        way_map[way_raw],
        float(data['rooms']),
        float(data['total_area']),
        float(data['storey']),
        float(data['storeys']),
        float(data['renovation']),
        float(data['building_age_years']),
    ]


def train_from_csv(path):
    """
    Обучает модель на указанном CSV и возвращает sklearn‑Pipeline.
    """
    data, labels = parser(path)
    model = build_model()
    model.fit(data, labels)
    return model


def test(model: Pipeline, test_l, test_d):
    """
    Простейшая печать предсказаний и средней абсолютной ошибки
    на тестовой выборке.
    """
    test_d = np.array(test_d, dtype=float)
    test_l = np.array(test_l, dtype=float)
    preds = model.predict(test_d)
    p = 0
    print('Predicted vs actual (price)')
    for pred, true in zip(preds, test_l):
        print(pred, true)
        p += abs(pred - true)
    p /= len(test_l)
    print('Mean absolute error:', p)


if __name__ == "__main__":
    default_train_path = Path(__file__).with_name('clean_train.csv')
    default_test_path = Path(__file__).with_name('clean_test.csv')

    train_data, train_labels = parser(str(default_train_path))
    test_data, test_labels = parser(str(default_test_path))

    model = build_model()
    model.fit(train_data, train_labels)

    test(model, test_labels, test_data)
