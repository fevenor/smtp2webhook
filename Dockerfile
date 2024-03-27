FROM golang:1.22.1 AS golang

# Build
ADD . /go/src/smtp2webhook
WORKDIR /go/src/smtp2webhook
RUN CGO_ENABLED=0 go build -o smtp2webhook -a -ldflags '-extldflags "-static"' .

FROM alpine
LABEL maintainer="fevenor <fevenor@outlook.com>"

# Copy bin
COPY --from=golang /go/src/smtp2webhook/smtp2webhook /usr/local/bin/smtp2webhook

# DDNS environment variables
ENV PORT 8587
ENV USERNAME user
ENV PASSWORD passwd
ENV WEBHOOK http://127.0.0.1:8080/notify?title={{.title}}&content={{.content}}

EXPOSE $PORT/tcp

# Start ddns server
CMD exec smtp2webhook
