# Этап сборки
FROM golang:1.24.3 as builder

WORKDIR /app
COPY . .


RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .


FROM ubuntu:latest

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /root/


COPY --from=builder /app/main .
COPY --from=builder /app/web ./web

ENV TODO_PORT=7540
ENV TODO_DBFILE=/data/scheduler.db
ENV TODO_PASSWORD=""
ENV TODO_JWT_SECRET="todo-secret-key-2024"


RUN mkdir -p /data

EXPOSE 7540

CMD ["./main"]