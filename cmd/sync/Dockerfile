FROM golang:1.21.5-alpine3.17 as build

WORKDIR /app

ADD . .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor

FROM alpine:3.17 as runner

COPY --from=build /app/sync /opt/sync

WORKDIR /opt

CMD [ "./sync" ]