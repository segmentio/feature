FROM golang:1.17 as build
WORKDIR /go/src/github.com/segmentio/feature
COPY . .
RUN CGO_ENABELD=0 go build -mod=vendor ./cmd/feature

FROM debian
COPY --from=build /go/src/github.com/segmentio/feature/feature /usr/local/bin/feature
VOLUME /var/run/feature/feature.db
ENV FEATURE_PATH=/var/run/feature/feature.db
ENTRYPOINT ["feature"]
