FROM golang:1.11-alpine as builder
RUN apk update && apk add git
WORKDIR /home/arriba/
COPY . .
RUN CGO_ENABLED=0 go build

FROM scratch
LABEL maintainer="Alfonso Acosta <fons@syntacticsugar.consulting>"
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /home/arriba/arriba /home/arriba/arriba
ENTRYPOINT ["/home/arriba/arriba"]
