FROM golang:1.10 as builder
MAINTAINER itsdalmo
ADD . /go/src/github.com/itsdalmo/ssm-sh
WORKDIR /go/src/github.com/itsdalmo/ssm-sh
ENV TARGET linux
ENV ARCH amd64
RUN make build-release

FROM alpine
RUN apk --no-cache add ca-certificates
ENTRYPOINT ["/bin/ssm-sh"]
CMD ["--help"]
COPY --from=builder /go/src/github.com/itsdalmo/ssm-sh/ssm-sh-linux-amd64 /bin/ssm-sh
