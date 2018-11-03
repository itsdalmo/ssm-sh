FROM golang:1.11 as builder

ADD . /go/src/github.com/itsdalmo/ssm-sh
WORKDIR /go/src/github.com/itsdalmo/ssm-sh
ARG TARGET=linux
ARG ARCH=amd64
RUN make build-release

FROM alpine:latest as resource
COPY --from=builder /go/src/github.com/itsdalmo/ssm-sh/ssm-sh-linux-amd64 /bin/ssm-sh
RUN apk --no-cache add ca-certificates
ENTRYPOINT ["/bin/ssm-sh"]
CMD ["--help"]

FROM resource
