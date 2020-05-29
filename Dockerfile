# torrentstream.io streaming application
FROM golang:latest AS builder
ENV APPDIR /go/src/app

RUN mkdir -p "$APPDIR"
ADD . $APPDIR
WORKDIR $APPDIR

# RUN go get .
RUN go build -o app .
# RUN rm -rf *.go
# omdb key
ENV OMDB 1d0bcf4c

# app flags and variables
EXPOSE 8080/tcp
EXPOSE 5000/udp
ENTRYPOINT ["./app", "-dir=torrents"]
