# torrentstream.io streaming application

# -- builder
FROM golang:go1.23.5-alpine AS builder
RUN apk add build-base
WORKDIR /go/src
COPY . .
RUN go build -o app /go/src/*.go

# -- app img
FROM alpine AS base
RUN apk add build-base
ADD www /www
COPY --from=builder /go/src/app /app

ENV OMDB <your_omdb_code>
EXPOSE 8080/tcp
EXPOSE 5000/udp
ENTRYPOINT ["./app", "-idle=5m"]
