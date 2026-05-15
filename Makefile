.PHONY: build run test test-integration lint docker-up docker-down

build:
	go build -o bin/server ./cmd/payment-gateway

run: build
	./bin/server

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

test-integration:
	docker compose up --build -d --remove-orphans
	@echo "Waiting for API to be healthy..."
	@for i in $$(seq 1 60); do \
		if curl -sf http://localhost:8080/health > /dev/null 2>&1; then \
			break; \
		fi; \
		sleep 1; \
	done
	go test -tags integration -v -count=1 ./tests/...; EXIT_CODE=$$?; \
	docker compose down -v; \
	exit $$EXIT_CODE

docker-up:
	docker compose up --build --remove-orphans

docker-down:
	docker compose down -v
