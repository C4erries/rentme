# Prompt: выполнить пункт 15 (адекватизация ML-цены) для Rentme — чистая схема “рубли везде”

Ты один агент-разработчик. Цель — сделать ML-предсказание цены **адекватным для демо** и одновременно **полностью убрать legacy-названия** (`*_cents` и т.п.) из домена, API и фронта.

## Перед началом (обязательно)

Ознакомься с документацией и правилами проекта:
- `AGENTS.md` (архитектурные принципы и ограничения)
- `plan.md` (общая дорожная карта, пункт 15)
- `domain_model.md` (доменная модель)
- `demo.md` (демо-сценарии, которые должны проходить)
- `mlrent/problem.md` и `mlrent/readme.md` (контракт и контекст ML сервиса)
- `docker-compose.yml` и `README.md` (как запускать сервисы)

## Что считается “готово”

- Во всей системе единица денег — **целые рубли** (`RUB`).
- В публичном API и моделях **нет** полей/параметров с суффиксом `_cents`.
- ML цены адекватные (не “копейки”), есть нормализация города + клампы на backend.

## Ограничения

- Не добавляй новые крупные технологии/сервисы (только правки в существующих Go/FastAPI/CSV).
- Это будет breaking change для API. Ок: проект учебный, важнее чистота и демо. Делай всё в одном изменении: backend + frontend + seed.
- Домен `internal/domain` не должен зависеть от инфраструктуры.
- Логи — структурированные (`log/slog`), без `fmt.Println`.

## Основная причина бага (почему “слишком низко”)

ML обучается/отвечает в рублях, но часть кода трактует суммы как копейки и делит на 100 при отображении. Исправляем это радикально: **везде рубли** + переименование полей.

## Задача

### 1) Переименовать деньги в домене и API (без legacy)

Приведи к единой схеме для объявления:
- `rate_rub` — цена в рублях (число).
- `price_unit` — `night` или `month` (в зависимости от `rental_term`).

Сделай полный рефактор по проекту:
- Backend DTO/HTTP payloads: заменить
  - `nightly_rate_cents` -> `rate_rub`
  - `rate_cents` -> `rate_rub`
  - `recommended_price_cents` -> `recommended_price_rub`
  - `current_price_cents` -> `current_price_rub`
  - query params `price_min_cents`/`price_max_cents` -> `price_min_rub`/`price_max_rub`
- Frontend types и UI: заменить соответствующие поля/форматирование.
- Seed-данные: `backend/data/listings.json` привести к `rate_rub` + `price_unit` (и к реалистичным значениям).

Важно:
- Весь UI должен перестать делать `*100`/`/100`.
- Везде где есть валюта — `RUB` (в т.ч. `MLPricingEngine` сейчас кладёт `"USD"` — исправить).

### 2) ML-контракт: цены в рублях

Зафиксируй в `mlrent`:
- `recommended_price`, `current_price`, `diff` — рубли.
- Никаких масштабирований `*100`/`/100`.
- Обнови документацию `mlrent/readme.md` и примеры запросов/ответов.

### 3) Нормализация города

Сделай канонизацию города (минимум Москва/Краснодар):
- trim + lower/upper для сопоставления
- map: `Moscow`/`москва` -> `Москва`, `Krasnodar`/`краснодар` -> `Краснодар`

Логируй `city_raw` и `city_normalized`.

### 4) Клампы на backend (demo safety)

После ответа ML (в рублях) добавь post-processing:
- min/max коридоры по (`city_normalized`, `rental_term`) и дефолты по `rental_term`.
- конфиг через env (JSON или набор env).
- логировать:
  - `listing_id`
  - `city_raw`, `city_normalized`
  - `rental_term`
  - `ml_price_raw`, `ml_price_final`
  - `clamped`, `clamp_min`, `clamp_max`

### 5) Датасеты: правдоподобные диапазоны

Отредактируй CSV так, чтобы диапазоны выглядели реалистично (рубли):
- `short_term`: ~3_000–30_000 руб/ночь
- `long_term`: ~25_000–250_000 руб/месяц

### 6) Быстрая валидация

Добавь воспроизводимую проверку:
- либо набор `curl` запросов,
- либо небольшой тест/скрипт, который проверяет “не ниже минимума” по 4–6 профилям.

Smoke-профили:
- Москва, `short_term`: rooms=1, area=35–45 -> >= 3_000 руб
- Москва, `short_term`: rooms=3, area=90–120 -> >= 8_000 руб
- Краснодар, `short_term`: rooms=1, area~35 -> >= 2_000 руб
- Москва, `long_term`: rooms=1, area~35 -> >= 25_000 руб
- Москва, `long_term`: rooms=3, area~90 -> >= 60_000 руб

## Definition of Done

1) В `GET /api/v1/...` и `POST /api/v1/...` нет `*_cents`; используется `rate_rub`, `recommended_price_rub`, `current_price_rub`, `price_min_rub`, `price_max_rub`.
2) В UI “Рекомендованная цена” выглядит правдоподобно, без скрытого деления/умножения на 100.
3) Клампы работают и логируются структурированно.
4) Нормализация города работает минимум для Москва/Краснодар.
5) Демо поднимается через compose и сценарии из `demo.md` проходят.

## Где искать код

- ML API: `mlrent/main.py`
- Тренировка/признаки: `mlrent/ml.py`
- Backend ML адаптер: `backend/internal/infra/pricing/ml_pricing.go`
- Маппинг DTO/JSON: `backend/internal/app/dto/*`, `backend/internal/infra/http/gin/*`
- Frontend деньги/типы: `frontend/src/pages/*`, `frontend/src/types/*`, `frontend/src/hooks/*`
