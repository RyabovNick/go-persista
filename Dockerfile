FROM golang:1.21 as builder

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOSUMDB=off
ENV GOMODCACHE=/go/build/.go/pkg/mod

WORKDIR /go/build

COPY . .
RUN go build -o go-persista ./cmd

FROM scratch

WORKDIR /srv

COPY --from=builder /go/build/go-persista ./

WORKDIR /recover

EXPOSE 8080
CMD ["/srv/go-persista"]