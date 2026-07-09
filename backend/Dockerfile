FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/otasign-backend ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /out/otasign-backend /app/otasign-backend
COPY db/migrations /app/db/migrations

EXPOSE 8080

CMD ["/app/otasign-backend"]
