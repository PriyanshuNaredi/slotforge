# --- Build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/slotforge-api ./cmd/api

# --- Runtime stage ---
FROM alpine:3.21
RUN addgroup -S app && adduser -S -G app -u 10001 app
COPY --from=build /out/slotforge-api /app/slotforge-api
USER app
WORKDIR /app
# Render/Railway inject PORT; APP_PORT default keeps local runs on 8080.
CMD ["sh", "-c", "APP_PORT=${PORT:-8080} exec ./slotforge-api"]
