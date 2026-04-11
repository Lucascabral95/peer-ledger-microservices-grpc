FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/grpc-ecosystem/grpc-health-probe@v0.4.38

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o user-service ./services/user-service/cmd/api

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/user-service .
COPY --from=builder /go/bin/grpc-health-probe /bin/grpc_health_probe

EXPOSE 50051

CMD ["./user-service"]
