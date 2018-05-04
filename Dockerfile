# ----------------------------------
# build the entrypoint binary
# ----------------------------------
FROM golang:1.9 AS go-build

ENV GOPATH /go
RUN mkdir -p /go/src && mkdir -p /go/bin

WORKDIR /go/src/github.com/mailgun/k8-entrypoint
COPY . .
RUN go install -ldflags "-linkmode external -extldflags -static" github.com/mailgun/k8-entrypoint/cmd/k8-entrypoint && \
    go install -ldflags "-linkmode external -extldflags -static" github.com/mailgun/k8-entrypoint/cmd/print-env


# ----------------------------------
# Basic container
# ----------------------------------
FROM scratch

# Copy the k8-entrypoint binary
COPY --from=go-build /go/bin/k8-entrypoint /usr/sbin/k8-entrypoint
COPY --from=go-build /go/bin/print-env /usr/sbin/print-env

CMD ["/usr/sbin/k8-entrypoint", "/usr/sbin/print-env"]
