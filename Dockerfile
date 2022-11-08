FROM golang:alpine

WORKDIR /build
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN go build -o app

FROM alpine:latest
LABEL org.opencontainers.image.source https://github.com/keukentrap/secure-send
# I know, this is bad, but docker is terrible
ENV SMTP_PASS="Eelco!"
WORKDIR /app
COPY --from=0 /build/app .
COPY templates/ ./templates/
COPY static/ ./static/
RUN chown -R 1000:1000 .
USER 1000:1000

ENTRYPOINT ./app
