FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o ./app cmd/main.go 

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/web ./web
COPY --from=builder /app/.env .env
COPY --from=builder /app/./app ./app
COPY --from=builder /app/cert.crt ./cert.crt
COPY --from=builder /app/Key.key ./Key.key
COPY --from=builder /app/LiberationSans-Bold.ttf ./LiberationSans-Bold.ttf

EXPOSE 8080
EXPOSE 8443

CMD ["./app"]