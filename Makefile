.PHONY: run build migrate-up migrate-down

# jalankan server lokal
run:
	go run main.go

# build binary ke folder bin/
build:
	go build -o bin/backend main.go

# jalankan migration (butuh DATABASE_URL di .env)
migrate-up:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate -path db/migrations -database "$$DATABASE_URL" up

migrate-down:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate -path db/migrations -database "$$DATABASE_URL" down 1
