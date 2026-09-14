FROM golang:1.24.2-alpine3.20 as build

WORKDIR /app

ADD . .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor

FROM alpine:3.20 as runner

COPY --from=build /app/sync /opt/sync

WORKDIR /opt

CMD [ "./sync" ]