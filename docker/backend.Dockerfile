FROM golang:1.24.3-alpine AS build

WORKDIR /src

COPY apps/backend/go.mod apps/backend/go.sum ./
RUN go mod download

COPY apps/backend/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api ./cmd/api

FROM alpine:3.20

RUN adduser -D -g '' appuser
USER appuser
WORKDIR /app

COPY --from=build /out/api /app/api

EXPOSE 8080

ENTRYPOINT ["/app/api"]
