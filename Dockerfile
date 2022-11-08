FROM golang:alpine

WORKDIR /build
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN go build -o app

FROM alpine:latest
LABEL org.opencontainers.image.source https://github.com/keukentrap/secure-send
# I know, this is bad, but docker is terrible
ARG SMTP_PASS
ENV SMTP_PASS=$SMTP_PASS
ARG GIT_SHA="-"
WORKDIR /app
COPY --from=0 /build/app .
COPY templates/ ./templates/
COPY static/ ./static/
RUN sed -ie "s/||SHA||/$GIT_SHA/g" templates/base.gohtml
RUN chown -R 1000:1000 .
USER 1000:1000

ENTRYPOINT ./app
