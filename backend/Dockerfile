FROM golang:1.24 AS builder

WORKDIR /src/backend

# Backend module files (the Docker build context is repo root).
COPY backend/go.mod backend/go.sum ./

# Include local messaging-service module for the replace directive used by the backend.
COPY messaging-service /src/messaging-service

RUN go mod download

COPY backend/. .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/rentme ./cmd/rentme

FROM debian:12-slim
WORKDIR /app
COPY --from=builder /out/rentme /app/rentme
COPY --from=builder /src/backend/data/listings.json /app/data/listings.json
EXPOSE 8080
ENV APP_ENV=prod \
    HTTP_ADDR=:8080 \
    MONGO_URI=mongodb://mongo:27017 \
    MONGO_DB=rentals
ENTRYPOINT ["/app/rentme"]
