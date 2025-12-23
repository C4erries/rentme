## ML‑сервис ценообразования для Rentme

Этот каталог содержит учебный ML‑сервис, который по признакам квартиры возвращает оценочную стоимость аренды.  
Сервис работает поверх подготовленных CSV‑датасетов и общается с Go‑backend по HTTP.

Основная цель: построить понятную связку «доменная модель Listing ↔ признаки ML ↔ HTTP‑адаптер», причём **отдельно для посуточной и долгосрочной аренды**.

---

### 1. Текущая схема данных (CSV)

Используются два файла в `mlrent/`:

- `clean_train.csv` — обучающая выборка;
- `clean_test.csv` — тестовая выборка.

Схема строк:

```text
id,city,price,minutes,way,rooms,total_area,storey,storeys,renovation,building_age_years
```

Поля:
- `id` — технический идентификатор строки;
- `city` — город (`Listing.Address.City`);
- `price` — целевая стоимость (целые рубли; в интеграции соответствует `Listing.RateRub`);
- `minutes` — примерное время до центра города, в минутах;
- `way` — способ добирания до центра (`walk` / `car`);
- `rooms` — количество комнат (`Listing.Bedrooms`);
- `total_area` — общая площадь (`Listing.AreaSquareMeters`);
- `storey` — этаж (`Listing.Floor`);
- `storeys` — этажность (`Listing.FloorsTotal`);
- `renovation` — качество ремонта 0–10 (`Listing.RenovationScore`);
- `building_age_years` — возраст дома в годах (`Listing.BuildingAgeYears`).

Признаки `minutes` и `way` сейчас могут подставляться заглушками на стороне backend‑адаптера (`MLPricingEngine`), но в идеале должны вычисляться по геоданным (расстояние до центра → время пешком/на машине).

---

### 2. Связь с доменной моделью Rentme

Целевая сущность на стороне backend — `Listing` (см. `domain_model.md` и `backend/internal/domain/listings/listing.go`).

Маппинг:
- `city` ← `Listing.Address.City`
- `rooms` ← `Listing.Bedrooms`
- `total_area` ← `Listing.AreaSquareMeters`
- `storey` ← `Listing.Floor`
- `storeys` ← `Listing.FloorsTotal`
- `renovation` ← `Listing.RenovationScore`
- `building_age_years` ← `Listing.BuildingAgeYears`
- `price` ↔ желаемая/наблюдаемая стоимость (используется как таргет при обучении)
- `minutes`, `way` ← производные признаки (время и способ добирания).

Отдельно в `Listing` хранится `RentalTermType` (`short_term`/`long_term`).  
ML‑сервис напрямую о структуре `Listing` не знает, но backend может передавать тип аренды в запросе и выбирать соответствующую модель.

---

### 3. HTTP‑API ML‑сервиса

Сервис написан на FastAPI (`mlrent/main.py`).

Основные endpoint'ы:
- `GET /health` - проверка живости.
- `POST /predict` - выдаёт рекомендованную цену.

Все цены в запросе/ответе — **RUB (целые рубли)**.

Пример запроса:

```json
{
  "listing_id": "uuid",
  "rental_term": "long_term",
  "city": "Moscow",
  "minutes": 20,
  "way": "car",
  "rooms": 2,
  "total_area": 55.0,
  "storey": 7,
  "storeys": 16,
  "renovation": 7,
  "building_age_years": 15,
  "current_price": 90000.0
}
```

Пример ответа:

```json
{
  "listing_id": "uuid",
  "recommended_price": 95000.0,
  "current_price": 90000.0,
  "diff": 5000.0
}
```

Backend‑адаптер (`MLPricingEngine`) преобразует `recommended_price` в `PriceBreakdown.Nightly` и считает итоговую стоимость брони.

---

### 4. Разделение моделей для short_term и long_term

Бизнес‑требования:
- есть два сегмента:
  - посуточная аренда (`short_term`);
  - долгосрочная аренда (`long_term`);
- **для каждого сегмента нужна своя ML‑модель и свой датасет**;
- UI может показывать рекомендации как для посуточной, так и для долгосрочной аренды (в зависимости от типа объявления).

Целевая схема:
- два набора данных:
  - `clean_train_short.csv` / `clean_test_short.csv`;
  - `clean_train_long.csv` / `clean_test_long.csv`;
- две модели в `mlrent/ml.py`:
  - `short_term_model`
  - `long_term_model`
- HTTP‑интерфейс:
  - один endpoint `/predict` с полем `rental_term` (`"short_term" | "long_term"`),  
    **или** два endpoint’а (`/predict/short`, `/predict/long`).

Интеграция:
- backend всегда передаёт `rental_term` согласно `Listing.RentalTermType`;
- `MLPricingEngine` выбирает модель/endpoint по этому полю;
- результат для обоих типов используется одинаково: как рекомендованная цена, которую хост может принять или игнорировать.

---

### 5. Встраивание в backend (`MLPricingEngine`)

На стороне Go:
- интерфейс `pricing.Calculator` определяет вход (`QuoteInput`) и выход (`PriceBreakdown`);
- реализация `MLPricingEngine`:
  - получает `Listing` по `ListingID`;
  - подготавливает JSON‑payload по схеме выше;
  - указывает `rental_term` (`short_term`/`long_term`);
  - отправляет запрос в ML‑сервис;
  - логирует ошибки и таймауты;
  - мапит `recommended_price` → `PriceBreakdown.Nightly`.

Особенности:
- при недоступности ML‑сервиса должен быть fallback (например, `memory`‑калькулятор);
- рекомендованная цена — **подсказка**, а не обязательное значение: хост всегда может задать свою цену;
- для обоих типов аренды (short/long) сценарий работает одинаково, различается только модель и, возможно, шкала/размерность цены.

---

### 6. Docker‑образ и интеграция в compose

Файл `mlrent/Dockerfile` описывает контейнер ML‑сервиса.  
Цель — запускать его вместе с остальными сервисами через корневой `docker-compose.yml`:

- сервис `rentme-ml` слушает, например, на `:8000`;
- backend получает URL через `ML_PRICING_URL`;
- при `PRICING_MODE=ml` backend использует именно ML‑калькулятор.

Точный статус интеграции и следующие шаги описаны в:
- `mlrent/plan.md` — план развития ML‑части;
- `plan.md` в корне — общий план проекта.
