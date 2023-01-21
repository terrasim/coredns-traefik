FROM golang:1.19-alpine as build

WORKDIR /coredns

COPY . .

RUN apk add --no-cache binutils

RUN cd coredns && \
    go build && \
    strip coredns

FROM alpine:3.17

WORKDIR /coredns

COPY --from=build /coredns/coredns/coredns /coredns/coredns
COPY ./entrypoint.sh .

RUN mkdir /etc/coredns

VOLUME ["/etc/coredns"]

EXPOSE 53 53/udp

ENTRYPOINT ["./entrypoint.sh"]
