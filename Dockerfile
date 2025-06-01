FROM golang:1.23.7

RUN apt-get update && \
    apt-get install -y python3 nodejs g++ openjdk-17-jdk

WORKDIR /app
COPY . .

RUN go build -o server

CMD ["./server"]
