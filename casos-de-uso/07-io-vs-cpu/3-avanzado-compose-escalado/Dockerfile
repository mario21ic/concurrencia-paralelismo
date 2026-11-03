# syntax=docker/dockerfile:1
# Se construye desde la RAÍZ del repositorio (allí está go.mod); lo hace compose.yaml.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY casos-de-uso/07-io-vs-cpu/3-avanzado-compose-escalado ./casos-de-uso/07-io-vs-cpu/3-avanzado-compose-escalado
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/escalado ./casos-de-uso/07-io-vs-cpu/3-avanzado-compose-escalado

FROM alpine:3.23
RUN adduser -D -u 10001 app
COPY --from=build /out/escalado /app/escalado
USER app
ENTRYPOINT []
CMD ["/app/escalado", "-modo=servidor"]
