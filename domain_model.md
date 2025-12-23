## Доменная модель Rentme (high‑level)

Документ фиксирует целевую доменную модель Rentme с учётом:
- разделения краткосрочной (`short_term`) и долгосрочной (`long_term`) аренды;
- наличия отдельного ML‑сервиса ценообразования;
- вынесения мессенджера в отдельный сервис (gRPC + ScyllaDB);
- отсутствия событийной архитектуры и Kafka.

Backend реализован как модульный монолит:
- `internal/domain` — чистые сущности и бизнес‑правила;
- `internal/app` — use‑cases, команды/запросы, обработчики;
- `internal/infra` — HTTP, Mongo, адаптеры ML/S3/Scylla, логирование.

---

### 1. Пользователь (User)

Файл: `backend/internal/domain/user/user.go`

`User`:
- `ID`
- `Email`, `Name`
- `PasswordHash`
- `Roles []Role` (`guest`, `host`, `admin`)
- `CreatedAt`, `UpdatedAt`

Роли:
- `guest` — ищет жильё, создаёт брони, пишет отзывы, общается в чате.
- `host` — создаёт и управляет объявлениями.
- `admin` — имеет доступ к админке, метрикам и может писать любому пользователю.

---

### 2. Объявление (Listing)

Файл: `backend/internal/domain/listings/listing.go`

Идентичность и владелец:
- `ID` (`ListingID`)
- `HostID` — арендодатель; **для каждого объявления обязательно задан**, чтобы гостю было понятно, кому писать.

Базовые поля:
- `Title`, `Description`
- `PropertyType` (квартира, комната, дом и т.п.)
- `Tags`, `Highlights`

Адрес:
- `Address`:
  - `Line1`, `Line2`
  - `City` — город
  - `Region` — регион / субъект РФ (вместо «Country» в пользовательских сценариях)
  - `Lat`, `Lon`

Параметры жилья:
- `GuestsLimit`
- `MinNights`
- `MaxNights`:
  - может быть **не задан** (например, `0` или `null` = «неограниченно»);
  - валидация не должна считать это ошибкой (`ErrNightsRange` нужно учитывать).
- `Bedrooms`, `Bathrooms`
- `AreaSquareMeters`
- `Floor`, `FloorsTotal`
- `RenovationScore` (0–10)
- `BuildingAgeYears` (или альтернативно `YearBuilt`)

Тип аренды:
- `RentalTermType`:
  - `short_term` — посуточная;
  - `long_term` — долгосрочная.
- От `RentalTermType` зависят:
  - правила доступности;
  - выбор ML‑модели;
  - часть UX в мастере объявления.

Правила:
- `HouseRules`
- `CancellationPolicyID`

Цены и рейтинг:
- `RateRub` — базовая ставка в рублях.
- `PriceUnit` — единица тарифа: `night` (для `short_term`) или `month` (для `long_term`).
- `Rating` - агрегированный рейтинг из отзывов.

Медиа и состояние:
- `ThumbnailURL`
- `Photos []string` / список URL (S3/MinIO)
- `AvailableFrom`
- `State` (`ListingDraft`, `ListingActive`, `ListingSuspended`)
- `CreatedAt`, `UpdatedAt`, `Version`

Публикация:
- Для перехода в `ListingActive` требуется:
  - валидный адрес (`City` + `Region` + `Line1`);
  - валидный диапазон ночей (`MinNights`, `MaxNights` с учётом «безлимита»);
  - заполненные обязательные поля по типу аренды.

---

### 3. Ценообразование (Pricing)

Файл: `backend/internal/domain/pricing/pricing.go`

`PriceBreakdown`:
- `Nights`
- `Nightly` (money)
- `Fees`, `Taxes`, `Discounts`
- `Total`

`QuoteInput`:
- `ListingID`
- `Range` (`DateRange`)
- `Guests`

Интерфейсы:
- `Calculator`:
  - `Quote(ctx, QuoteInput) (PriceBreakdown, error)`
- `PricingPort` (в `internal/app`):
  - скрывает конкретную реализацию калькулятора (`memory` / ML).

ML‑интеграция:
- адаптер `MLPricingEngine` (`internal/infra/pricing/ml_pricing.go`):
  - загружает `Listing` по `ListingID`;
  - формирует JSON‑запрос к ML‑сервису:
    - `city` ← `Listing.Address.City`;
    - `rooms` ← `Bedrooms`;
    - `total_area` ← `AreaSquareMeters`;
    - `storey` ← `Floor`;
    - `storeys` ← `FloorsTotal`;
    - `renovation` ← `RenovationScore`;
    - `building_age_years` ← `BuildingAgeYears`;
    - `minutes`, `way` — производные признаки (пока могут задаваться константой, в будущем по геоданным);
    - `rental_term` — тип аренды (`short_term`/`long_term`).
  - вызывает `/predict` (или специализированный endpoint);
  - получает `recommended_price` и мапит его в `PriceBreakdown.Nightly`.

Важно:
- есть **две ML‑модели** и две группы датасетов:
  - `short_term_model` — для посуточной аренды;
  - `long_term_model` — для долгосрочной;
- ML‑подсказки используются **и для short_term, и для long_term**, но каждая модель обучается на «своём» сегменте.

При недоступности ML:
- `MLPricingEngine` логирует ошибку и может отдавать ошибку наружу;
- в конфигурации возможно переключение на `memory`‑калькулятор (fallback).

---

### 4. Доступность (Availability)

Файлы: `backend/internal/domain/availability/*`

Задачи контекста:
- хранение интервалов доступности/занятости для каждого объявления;
- проверка возможности бронирования на заданный диапазон дат;
- учёт типа аренды (`short_term`/`long_term`) при поиске слотов.

Контекст взаимодействует с `booking` и `pricing`:
- перед созданием брони проверяется доступность;
- при расчёте цены используется `DateRange` из доступности.

---

### 5. Бронирование (Booking)

`Booking` (целевой дизайн):
- `ID`
- `ListingID`
- `GuestID`
- `HostID`
- `Range` (`DateRange`)
- `GuestsCount`
- `Price` (снимок `PriceBreakdown` / итоговая сумма)
- `Status`:
  - `Pending`
  - `Confirmed`
  - `Cancelled`
  - `Completed`
- `CreatedAt`, `UpdatedAt`

Правила:
- бронь нельзя создать, если объявление недоступно;
- цену брони нужно считать через `pricing.Calculator`;
- переходы статусов должны быть явными и проверяемыми.

UX:
- кнопка «забронировать» должна вести к созданию `Booking`, а не просто показывать абстрактное уведомление.

---

### 6. Отзывы (Reviews)

Файлы: `backend/internal/domain/reviews/*`

`Review`:
- `ID`
- `ListingID`
- `AuthorID`
- `Rating` (1–5)
- `Comment`
- `CreatedAt`

Агрегация:
- `Listing.Rating` — вычисляется из отзывов и используется:
  - в каталоге (сортировка/фильтры);
  - в админке и, при необходимости, в анализе качества ML‑модели.

---

### 7. Мессенджер (Messaging) и ScyllaDB

Мессенджер логически связан с доменом проекта, но реализуется отдельным сервисом:
- `messaging-service` — отдельный процесс/контейнер;
- общается с основным backend по gRPC;
- хранит историю сообщений **только** в ScyllaDB;
- основной backend не пишет напрямую в Scylla и работает только с Mongo.

Модель данных (логическая):

`Conversation`:
- `ID`
- `ParticipantIDs` (guest, host, admin)
- `ListingID` (опционально) — связанное объявление;
- `BookingID` (опционально) — связанная бронь;
- `CreatedAt`

`Message`:
- `ID`
- `ConversationID`
- `AuthorID`
- `Body`
- `SentAt`
- признак прочтения / другие метаданные.

Роли в мессенджере:
- `guest ↔ host` — общение по конкретному объявлению или брони;
- `admin ↔ любой пользователь` — такие же диалоги, но инициируются из админки.

На стороне основного backend:
- HTTP‑API для фронта проксирует/оркестрирует вызовы к gRPC‑сервису `messaging`;
- есть методы:
  - получить список диалогов текущего пользователя;
  - получить сообщения конкретного диалога;
  - отправить сообщение (создать диалог при необходимости).

---

### 8. Уведомления (Notifications)

Контекст уведомлений **не** строится на Kafka или событийной шине.  
Вместо этого:
- уведомления инициируются напрямую из use‑cases (в `internal/app`):
  - создание/подтверждение/отмена `Booking`;
  - новые сообщения в `Conversation`;
  - изменение статуса `Listing`;
- каналы:
  - email/SMS (для учебного проекта могут быть заглушки);
  - внутренние уведомления в UI (например, значки непрочитанных сообщений).

В будущем можно добавить очередь/брокер, но это не является целью учебного проекта.

---

### 9. Админка и метрики ML‑моделей

Админский функционал:
- список пользователей:
  - фильтр по ролям;
  - поиск по email;
  - выбор пользователя и открытие с ним диалога **в том же мессенджере**, что и у обычных пользователей;
- доступ к просмотру объявлений и броней (для модерации).

Метрики ML:
- базовые метрики качества:
  - MAE/MAPE на `clean_test.csv` (отдельно для short/long);
  - распределение ошибок/цен;
  - количество запросов к ML‑сервису.
- источник:
  - либо отдельный endpoint ML‑сервиса (например, `/metrics`);
  - либо логирование при старте/обучении, которое админка может парсить/визуализировать.

---

### 10. Связь ML с CSV‑датасетами

Текущая базовая схема CSV (`mlrent/clean_train.csv`, `mlrent/clean_test.csv`):

```text
id,city,price,minutes,way,rooms,total_area,storey,storeys,renovation,building_age_years
```

Маппинг в домен:
- `city` ↔ `Listing.Address.City`
- `rooms` ↔ `Listing.Bedrooms`
- `total_area` ↔ `Listing.AreaSquareMeters`
- `storey` ↔ `Listing.Floor`
- `storeys` ↔ `Listing.FloorsTotal`
- `renovation` ↔ `Listing.RenovationScore`
- `building_age_years` ↔ `Listing.BuildingAgeYears`
- `price` ↔ целевая стоимость (в рублях; как правило, трактуемая как `RateRub`)
- `minutes`, `way` ↔ производные признаки (время и способ добирания до центра), которые должны вычисляться по геоданным.

Разделение short/long:
- данные и модели должны быть разделены логически:
  - наборы строк, относящиеся к `short_term`, обучают `short_term_model`;
  - наборы для `long_term` — `long_term_model`;
- backend при запросе цены:
  - передаёт `rental_term`;
  - `MLPricingEngine` выбирает соответствующую модель/endpoint;
  - UI может показывать рекомендованную цену и для посуточной, и для долгосрочной аренды.
