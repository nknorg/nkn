FROM golang:1.12.4-alpine as builder
LABEL maintainer="gdmmx@nkn.org"
RUN apk add make git curl
ADD . /go/src/github.com/nknorg/nkn
WORKDIR /go/src/github.com/nknorg/nkn
RUN make glide
RUN make vendor
RUN make

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/github.com/nknorg/nkn/nknd /nkn/
COPY --from=builder /go/src/github.com/nknorg/nkn/nknc /nkn/
COPY --from=builder /go/src/github.com/nknorg/nkn/config.testnet.json /nkn/
RUN ln -s /nkn/nknd /nkn/nknc /usr/local/bin/
WORKDIR /nkn/data
CMD nknd
