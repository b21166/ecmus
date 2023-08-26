FROM golang:alpine AS build-env
RUN apk --no-cache add build-base git mercurial gcc
ADD . /src
RUN cd /src && go mod tidy
RUN cd /src && go build -o ecmus

FROM alpine:latest
WORKDIR /app
COPY --from=build-env /src/config /app/
COPY --from=build-env /src/ecmus /app/
COPY --from=build-env /src/config.yaml /app/
ENTRYPOINT ["./ecmus", "--config_file", "config.yaml"]

