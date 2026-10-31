# Atajos. Ejecutar desde proyecto-final/.
.PHONY: test race bench build up escalar carga down logs pprof

test:
	go vet ./... && go test -race -count=1 ./...

race:
	go test -race -count=5 ./...

bench:
	go test -run xxx -bench . -benchmem -cpu 1,4,8 ./internal/planificador/

build:
	go build -o bin/ ./cmd/...

up:
	docker compose up -d --build

# make escalar N=4
escalar:
	docker compose up -d --scale worker=$(N)

carga:
	docker compose run --rm loadgen

down:
	docker compose down

logs:
	docker compose logs -f --tail=50

# Perfil de CPU del coordinador durante 10 s (mientras corre la carga)
pprof:
	go tool pprof -top -seconds 10 http://localhost:8080/debug/pprof/profile
