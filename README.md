# Rentme - маркетплейс аренды жилья с ML-прайсингом

Rentme - учебный маркетплейс краткосрочной и долгосрочной аренды жилья (аналог Airbnb) с отдельным ML-сервисом ценообразования, чатом host - guest и административными инструментами. Архитектура строится как модульный монолит на Go с готовностью к выделению микросервисов.

## Стек
- Backend: Go + Gin, MongoDB, модульный монолит по DDD каталогам (`internal/domain`, `internal/app`, `internal/infra`).
- Мессенджер: отдельный gRPC-сервис (Go) + ScyllaDB, подключение из backend по gRPC.
- ML-прайсинг: FastAPI + scikit-learn (`mlrent/`).
- Frontend: React + TypeScript + Vite + Tailwind.
- Хранилище файлов: MinIO (S3-совместимое).
- Orchestration: `docker-compose` (backend, frontend, mlpricing, messaging-service, mongo, minio, scylla). Kafka больше не используется.
- Тарифы: для `short_term` цена указывается за ночь, для `long_term` — за месяц (`price_unit` в API; `nightly_rate_cents` остаётся для совместимости).

## Как запустить
Самый простой способ - поднять всё через Docker Compose:

```bash
docker-compose up
```

Сервисы: `rentme` (backend), `frontend`, `mlpricing`, `messaging-service`, `mongo`, `minio`, `scylla`. Все сервисы общаются внутри сети `rentme-net`; фронт доступен на http://localhost:3000, backend - http://localhost:8080/api/v1.
Демо-данные: при `APP_ENV=dev` или `DEMO_SEED=1` backend автоматически создаёт demo-аккаунты (см. `demo.md`) и подхватывает объявления из `backend/data/listings.json`.

Локально без контейнеров:
- Backend: `cd backend && go run ./cmd/rentme` (нужны переменные окружения для Mongo/MinIO/messaging).
- Frontend: `cd frontend && npm install && npm run dev`.
- ML-сервис: `cd mlrent && uvicorn app:app --reload --port 8000`.

## Полезные файлы
- `AGENTS.md` - правила для архитектуры и изменений.
- `plan.md` - дорожная карта задач.
- `domain_model.md` - описание основных сущностей.
- `mlrent/readme.md` - заметки по ML-сервису.
