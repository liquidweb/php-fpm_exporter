FROM golang:1.12-alpine as gobuilder

RUN apk update && apk add git

WORKDIR /src/
COPY . .
RUN CGO_ENABLED=0 go build

FROM scratch
COPY --from=gobuilder /src/php-fpm_exporter /app/php-fpm_exporter
WORKDIR /app
EXPOSE 9253/tcp
ENTRYPOINT ["/app/php-fpm_exporter"]
CMD ["server"]
