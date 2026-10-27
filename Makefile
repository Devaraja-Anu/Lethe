.PHONY: db-up db-down db-logs migrate fixtures test test-integration

DB_URL ?= postgres://lethe:lethe_password@localhost:5432/lethe_test?sslmode=disable

db-up:
	docker compose up -d
	@echo "Waiting for PostgreSQL to become healthy..."
	@docker compose exec postgres pg_isready -U lethe -d lethe_test

db-down:
	docker compose down -v

db-logs:
	docker compose logs -f postgres

migrate:
	docker compose exec -T postgres psql -U lethe -d lethe_test < migrations/001_create_lethe_tables.sql

fixtures:
	docker compose exec -T postgres psql -U lethe -d lethe_test < testdata/schema.sql
	docker compose exec -T postgres psql -U lethe -d lethe_test < testdata/fixtures.sql

test:
	go test -v ./...

test-integration:
	go test -v -tags=integration ./...
