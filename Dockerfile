# torrentstream.io streaming application

# -- builder
FROM golang:1.14.1-alpine AS builder
RUN apk add build-base
WORKDIR /go/src
COPY . .
RUN go build -o app /go/src/*.go

# -- app img
FROM alpine AS base
RUN apk add build-base
ADD www /www
COPY --from=builder /go/src/app /app

ENV OMDB 1d0bcf4c
EXPOSE 8080/tcp
EXPOSE 5000/udp
ENTRYPOINT ["./app", "-dir=torrents"]
