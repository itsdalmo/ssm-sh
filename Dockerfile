FROM alpine
MAINTAINER itsdalmo

COPY ssm-sh-linux-amd64 /bin/ssm-sh

ENTRYPOINT ["/bin/ssm-sh"]
CMD ["--help"]
