ARG base
FROM golang:1.13.8-alpine as builder
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
COPY --from=builder /nkn/certs /nkn/certs
COPY --from=builder /nkn/web /nkn/web
COPY --from=builder /nkn/docker/entrypoint.sh /nkn/
RUN ln -s /nkn/nknd /nkn/nknc /usr/local/bin/
WORKDIR /nkn/data
ENTRYPOINT ["/nkn/entrypoint.sh"]
CMD ["nknd", "--password-file", "wallet.pswd", "--web-gui-listen-address", "0.0.0.0"]
