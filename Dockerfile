FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /app/server ./cmd/server/main.go
RUN go build -o /app/seed ./cmd/seed/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/seed .

#EXPOSE 50051

CMD ["./server"]