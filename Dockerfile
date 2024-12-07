# torrentstream.io streaming application

# -- builder
FROM golang:1.14.1-alpine AS builder
RUN apk add build-base
WORKDIR /go/src
COPY . .
RUN go build -o ts /go/src/*.go

# -- app img
FROM alpine AS base
RUN apk add build-base
ADD www /www
COPY --from=builder /go/src/ts /ts

ENV OMDB <your_omdb_key>
EXPOSE 8080/tcp
EXPOSE 5000/udp
ENTRYPOINT ["./app", "-idle=5m"]
