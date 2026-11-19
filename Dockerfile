FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/holacloud/clocme/config.VERSION=${VERSION}" -o /clocme .

FROM alpine:3.22
# git is required at runtime: clones and blob reads shell out to it.
RUN apk add --no-cache git ca-certificates && adduser -D -u 1000 clocme && mkdir /data && chown clocme /data
USER clocme
WORKDIR /app
COPY --from=build /clocme /app/clocme
ENV DATADIR=/data
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/app/clocme"]
