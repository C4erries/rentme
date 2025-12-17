FROM golang:1.22 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=auto

RUN go build -o /app/bin/messaging-service ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=builder /app/bin/messaging-service /usr/bin/messaging-service
EXPOSE 9000
ENTRYPOINT ["/usr/bin/messaging-service"]
