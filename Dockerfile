FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o alert-gateway ./cmd/server


FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/alert-gateway .

EXPOSE 8080

ENTRYPOINT ["./alert-gateway"]