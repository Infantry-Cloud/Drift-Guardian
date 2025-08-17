FROM tofuutils/tenv:latest as base

FROM golang:1.24.4 AS builder
WORKDIR /src
COPY *.go .

RUN go mod init drift-guardian && \
    go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o terraform .

FROM base

RUN mv $(which terraform) /usr/local/bin/terraform-bin

# Install dependencies
RUN apk update && \
    apk add aws-cli && \
    apk add helm

ENV TENV_AUTO_INSTALL=true
ENV TENV_QUIET=true

COPY --from=builder /src/terraform /usr/local/bin/terraform
RUN chmod +x /usr/local/bin/terraform
ENV TERRAFORM_BINARY=/usr/local/bin/terraform-bin
ENTRYPOINT ["terraform"]
CMD ["plan"]
