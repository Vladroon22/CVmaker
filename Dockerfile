FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o app ./cmd/main.go

FROM alpine:latest AS permissions

WORKDIR /app

COPY --from=builder /app/app /app/app
COPY --from=builder /app/web /app/web
COPY --from=builder /app/.env /app/.env
COPY --from=builder /app/cert.crt /app/cert.crt
COPY --from=builder /app/Key.key /app/Key.key
COPY --from=builder /app/ttf/LiberationSans-Bold.ttf /app/ttf/LiberationSans-Bold.ttf

RUN chmod 755 /app/app && \
    chmod 644 /app/.env /app/cert.crt /app/Key.key && \
    chmod -R 644 /app/web && \
    chmod -R 644 /app/ttf

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=permissions /app/web ./web
COPY --from=permissions /app/.env .env
COPY --from=permissions /app/app ./app
COPY --from=permissions /app/cert.crt ./cert.crt
COPY --from=permissions /app/Key.key ./Key.key
COPY --from=permissions /app/ttf/LiberationSans-Bold.ttf ./ttf/LiberationSans-Bold.ttf

USER nonroot:nonroot

EXPOSE 8080
EXPOSE 8443

CMD ["./app"]
