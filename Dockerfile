FROM golang:1.24.5 as build

COPY . .

ENV GOPATH=""
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
RUN go build -trimpath -v -a -o app -ldflags="-w -s"
RUN chmod +x go-goof

RUN useradd -u 12345 infantry-cloud

FROM scratch
COPY --from=build /go/app /drift-guardian
COPY --from=build /etc/passwd /etc/passwd
USER infantry-cloud
ENTRYPOINT ["/drift-guardian"]
