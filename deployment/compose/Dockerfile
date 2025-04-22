FROM golang:1.21.5

RUN go install github.com/ysmood/kit/cmd/guard@v0.25.11

ADD . /go/src/github.com/eclipse-xfsc/custom-policy-agent

WORKDIR /go/src/github.com/eclipse-xfsc/custom-policy-agent

EXPOSE 8080

ENTRYPOINT ["sh", "-c", "/go/bin/guard -w '**/*.go' -- go run -mod=vendor ./cmd/policy/..."]
