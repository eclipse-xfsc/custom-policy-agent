FROM golang:1.21.6-alpine3.19 as builder

RUN apk add git

WORKDIR /go/src/github.com/eclipse-xfsc/custom-policy-agent

ARG APP_REPO_TAG

ADD . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X main.Version=$APP_REPO_TAG" -mod=vendor -o /tmp/policy ./cmd/policy/...

FROM alpine:3.19 as runner

COPY --from=builder /tmp/policy /opt/policy

WORKDIR /opt

CMD ["./policy"]
