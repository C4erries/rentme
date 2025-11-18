FROM golang:1.24 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/rentme ./rentme

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/rentme /app/rentme
EXPOSE 8080
ENV APP_ENV=prod \
    HTTP_ADDR=:8080
ENTRYPOINT ["/app/rentme"]
