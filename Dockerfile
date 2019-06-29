ARG base
FROM golang:1.12.4-alpine as builder
RUN apk add make git curl
ADD . /nkn
WORKDIR /nkn
ARG build_args
RUN make $build_args

FROM ${base}alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /nkn/nknd /nkn/
COPY --from=builder /nkn/nknc /nkn/
COPY --from=builder /nkn/config.mainnet.json /nkn/
RUN ln -s /nkn/nknd /nkn/nknc /usr/local/bin/
WORKDIR /nkn/data
CMD nknd
