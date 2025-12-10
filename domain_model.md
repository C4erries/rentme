## Доменная модель Rentme (high‑level)

Файл фиксирует основные доменные сущности Rentme и те поля, которые важны для интеграции с ML‑сервисом ценообразования.

Backend построен как модульный монолит в стиле DDD:
- `internal/domain` — чистый домен и бизнес‑правила;
- `internal/app` — use case‑ы (команды/запросы), хендлеры, политики;
- `internal/infra` — HTTP, БД, брокер, адаптеры.

---

### 1. Пользователь (User)

Файл: `backend/internal/domain/user/user.go`

Основные поля:
- `ID` — идентификатор пользователя;
- `Email`, `Name`;
- `PasswordHash`;
- `Roles []Role` — роли (`guest`, `host` и др.);
- `CreatedAt`, `UpdatedAt`.

Роли:
- `guest` — гость/арендатор;
- `host` — владелец объявлений.

---

### 2. Объявление (Listing)

Файл: `backend/internal/domain/listings/listing.go`

Основной агрегат маркетплейса — объявление о сдаче жилья.

Ключевые поля:
- Идентификация и владелец:
  - `ID` (`ListingID`);
  - `Host` (`HostID`);
- Контент и тип:
  - `Title`, `Description`;
  - `PropertyType` — тип объекта (квартира, лофт, дом и т.п.);
  - `Tags`, `Highlights`;
- Локация:
  - `Address`:
    - `Line1`, `Line2`;
    - `City`, `Country`;
    - `Lat`, `Lon`.
- Характеристики размещения:
  - `GuestsLimit` — максимальное число гостей;
  - `MinNights`, `MaxNights` — ограничения по длительности бронирования;
  - `Bedrooms`, `Bathrooms`;
  - `AreaSquareMeters` — общая площадь (важна и для ML‑модели);
  - (планируемые поля) `Floor`, `FloorsTotal` — этаж и этажность дома (соответствуют CSV‑полям `storey`, `storeys`);
  - (планируемое поле) `RenovationScore` (0–10) — уровень ремонта (соответствует CSV‑полю `renovation`);
  - (планируемое поле) `BuildingAgeYears` **или** `YearBuilt` — возраст дома / год постройки (соответствует CSV‑полю `building_age_years`).
- Правила и политики:
  - `HouseRules` — правила проживания;
  - `CancellationPolicyID` — ID политики отмены;
- Цена и качество:
  - `NightlyRateCents` — цена за ночь в минимальных денежных единицах;
  - `Rating` — агрегированный рейтинг по отзывам;
- Медиа и жизненный цикл:
  - `ThumbnailURL`, `Photos`;
  - `AvailableFrom` — дата, с которой жильё доступно;
  - `State` (`ListingDraft`, `ListingActive`, `ListingSuspended`);
  - `CreatedAt`, `UpdatedAt`, `Version`.

Доменные события:
- `ListingCreatedEvent`, `ListingActivatedEvent`, `ListingSuspendedEvent`, `ListingUpdatedEvent`.

---

### 3. Ценообразование (Pricing)

Файл: `backend/internal/domain/pricing/pricing.go`

Контекст отвечает за расчёт стоимости проживания.

Основные сущности:
- `PriceBreakdown`:
  - `Nights` — количество ночей;
  - `Nightly` — ночная цена (`money.Money`);
  - `Fees []Fee` — дополнительные сборы;
  - `Taxes []Tax` — налоги;
  - `Discounts []Discount` — скидки;
  - `Total` — итоговая стоимость.
- `QuoteInput`:
  - `ListingID` — ссылка на объявление;
  - `Range` (`DateRange`) — период проживания;
  - `Guests` — количество гостей.
- `Calculator` (доменный интерфейс):
  - `Quote(ctx, input) (PriceBreakdown, error)`.

Инфраструктурные реализации:
- `internal/infra/storage/memory.PricingEngine` — детерминированный калькулятор для демо;
- в будущем — адаптер `MLPricingEngine`, использующий ML‑сервис для оценки `Nightly`.

`PriceBreakdown` сохраняется внутри бронирования и используется для аналитики/отчётности.

---

### 4. Бронирование (Booking)

Файл: `backend/internal/domain/booking/booking.go`

Агрегат, описывающий факт бронирования жилья.

Основные элементы:
- Идентификация и участники:
  - `ID`;
  - `ListingID`;
  - `GuestID`;
- Даты и состояние:
  - `DateRange` — даты заезда/выезда;
  - состояние бронирования (создано, подтверждено, отменено и т.п.);
- Цена:
  - `Price` (`pricing.PriceBreakdown`) — зафиксированная на момент бронирования стоимость;
- Временные метки и события:
  - момент создания/обновления;
  - доменные события для интеграций.

---

### 5. Доступность (Availability)

Файлы: `backend/internal/domain/availability/*`

Контекст описывает календарь доступности объявления:
- хранит занятые/свободные даты;
- используется при формировании доступных слотов для бронирования и в UI календаря;
- влияет на то, для каких дат контекст `pricing` и, косвенно, ML‑сервис будут считать цену (через `DateRange`), но сам по себе не является фичей ML‑модели.

---

### 6. Отзывы (Reviews)

Файлы: `backend/internal/domain/reviews/*`

Отвечает за отзывы гостей и рейтинги:
- отдельные оценки + текстовые комментарии;
- агрегированный рейтинг (`Rating`), который попадает в `Listing.Rating` и может использоваться ML‑сервисом как признак «качества» жилья.

---

### 7. Связь домена с ML‑сервисом цены

Для ML‑ценообразования особенно важны:
- из `Listing`:
  - `Address.City`;
  - `Bedrooms`, `Bathrooms`, `AreaSquareMeters`;
  - `Floor`, `FloorsTotal` (планируемые);
  - `RenovationScore`, `BuildingAgeYears`/`YearBuilt` (планируемые);
  - `NightlyRateCents`;
  - `Rating`;
- из `pricing`:
  - `PriceBreakdown.Nightly`, `PriceBreakdown.Total`;
  - интерфейс `Calculator`.

Типовой сценарий взаимодействия:
1. Клиент (гость или хост) инициирует расчёт цены/рекомендации.
2. Приложение вызывает порт `PricingPort`, который использует реализацию `pricing.Calculator`.
3. Адаптер `MLPricingEngine` считывает `Listing`, формирует фичи (в т.ч. по схеме CSV: `city`, `rooms`, `total_area`, `storey`, `storeys`, `renovation`, `building_age_years`, а также вычисляемые `minutes`, `way`).
4. ML‑сервис возвращает рекомендованную ночную цену, которая упаковывается в `PriceBreakdown` и используется дальше в домене.

Такое разделение позволяет:
- сохранить чистый домен в Go;
- развивать ML‑сервис независимо (отдельный процесс/контейнер);
- жёстко контролировать набор признаков: все фичи либо хранятся в домене, либо однозначно из него выводятся.

