## ML‑сервис оценки стоимости квартиры в Rentme

Этот каталог описывает ML‑подсистему, которая прогнозирует **рекомендованную ночную цену** для объявлений (`Listing`) в Rentme.

Задача: синхронизировать схему учебных CSV‑датасетов с доменной моделью и будущим HTTP‑сервисом, который будет использоваться контекстом `pricing`.

Подробная постановка задачи и связь с доменом описаны в `mlrent/problem.md` и `domain_model.md`. Ниже — краткое резюме по данным и интеграции.

---

### 1. Схема данных для обучения

Оба CSV‑файла (`clean_train.csv`, `clean_test.csv`) имеют структуру:

```text
id,city,price,minutes,way,rooms,total_area,storey,storeys,renovation,building_age_years
```

Интерпретация полей:
- `id` — индекс строки;
- `city` — город (учебно: `Moscow`);
- `price` — цена аренды (таргет, далее приводится к ночной ставке);
- `minutes` — время до центра города в минутах;
- `way` — тип пути до центра (`walk` — пешком, `car` — на машине/транспорте);
- `rooms` — количество комнат (будет использоваться как `Bedrooms`);
- `total_area` — общая площадь квартиры, м² (`AreaSquareMeters`);
- `storey` — этаж квартиры;
- `storeys` — этажность дома;
- `renovation` — уровень ремонта от 0 до 10;
- `building_age_years` — возраст дома в годах.

Все поведенческие и производные признаки убраны из CSV и при необходимости должны вычисляться в ML‑коде на основе этих базовых полей.

---

### 2. Связь с доменной моделью Rentme

Ключевые поля доменной модели `Listing` (см. `domain_model.md` и `backend/internal/domain/listings/listing.go`):

- Локация:
  - `Address.City`, `Address.Country`, `Address.Lat`, `Address.Lon`;
- Характеристики квартиры:
  - `Bedrooms`, `Bathrooms`;
  - `AreaSquareMeters`;
  - (план) `Floor`, `FloorsTotal`;
  - (план) `RenovationScore`, `BuildingAgeYears` или `YearBuilt`;
- Цена:
  - `NightlyRateCents`, `Rating`.

Соответствие CSV ↔ домен:
- `city` → `Address.City`;
- `rooms` → `Bedrooms`;
- `total_area` → `AreaSquareMeters`;
- `storey` → `Floor` (планируемое поле);
- `storeys` → `FloorsTotal` (планируемое поле);
- `renovation` → `RenovationScore` (планируемое поле);
- `building_age_years` → `BuildingAgeYears` или производная от `YearBuilt`;
- `price` → целевая цена, согласованная по масштабу с `NightlyRateCents`;
- `minutes`, `way` → признаки транспортной доступности (могут остаться чисто ML‑фичами).

---

### 3. Черновой HTTP‑контракт ML‑сервиса

ML‑сервис будет небольшим Python‑приложением, принимающим снапшот квартиры и отдающим рекомендованную ночную цену.

Эскиз эндпоинта:

- `POST /predict`
  - Вход:
    ```json
    {
      "listing_id": "string",
      "city": "Moscow",
      "price": 95000,
      "minutes": 20,
      "way": "walk",
      "rooms": 3,
      "total_area": 120.0,
      "storey": 7,
      "storeys": 25,
      "renovation": 8,
      "building_age_years": 10
    }
    ```
  - Выход:
    ```json
    {
      "listing_id": "string",
      "recommended_nightly_rate_cents": 8500,
      "confidence": 0.78
    }
    ```

Список полей входа совпадает с CSV‑схемой, чтобы обучающие данные и онлайн‑фичи были согласованы.

---

### 4. Интеграция с бэкендом Rentme

ML‑сервис встраивается через контекст `pricing`:

- доменный интерфейс `pricing.Calculator` и структура `PriceBreakdown`;
- порт `PricingPort` в `internal/app/policies`;
- хендлер `HostListingPriceSuggestionHandler`, отдающий хосту рекомендованную цену.

План интеграции (подробнее в `mlrent/plan.md`):
- реализовать адаптер `MLPricingEngine` в `internal/infra`, который:
  - по `QuoteInput{ ListingID, Range, Guests }` читает `Listing` из БД;
  - формирует запрос к `POST /predict` по CSV‑схеме;
  - использует предсказанную цену как `Nightly` в `PriceBreakdown`;
- зарегистрировать адаптер как реализацию `pricing.Calculator`/`PricingPort` рядом с существующим `memory.PricingEngine` (для dev‑режима).

