FROM golang:1.23.7

RUN apt-get update && \
    apt-get install -y python3 nodejs

WORKDIR /app
COPY . .

RUN go build -o server

CMD ["./server"]