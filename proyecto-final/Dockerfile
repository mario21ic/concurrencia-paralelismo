# syntax=docker/dockerfile:1
# Se construye desde la RAÍZ del repositorio (allí está go.mod):
#   docker build -f proyecto-final/Dockerfile -t jobq .

FROM golang:1.26-alpine AS build
WORKDIR /src
# Capa de dependencias: solo se invalida si cambian go.mod / go.sum.
COPY go.mod go.sum ./
RUN go mod download
COPY proyecto-final ./proyecto-final
# Binarios estáticos y pequeños (sin cgo).
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./proyecto-final/cmd/...

FROM alpine:3.23
RUN adduser -D -u 10001 jobq
COPY --from=build /out/ /app/
USER jobq
# Una sola imagen con los tres binarios: cada servicio de compose elige el suyo con `command`.
ENTRYPOINT []
CMD ["/app/coordinador"]
