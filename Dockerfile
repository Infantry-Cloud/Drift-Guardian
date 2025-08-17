FROM golang:1.24.4 AS builder

COPY . .

ENV GOPATH=""
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
RUN go build -trimpath -v -a -o drift-guardian -ldflags="-w -s"
RUN chmod +x drift-guardian

RUN useradd -u 12345 infantry-cloud

FROM scratch
COPY --from=builder /go/drift-guardian /drift-guardian
COPY --from=builder /etc/passwd /etc/passwd
USER infantry-cloud
ENTRYPOINT ["/drift-guardian"]

