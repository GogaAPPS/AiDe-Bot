.PHONY: run test check fmt fmt-check build

run:
	@if [ -f .env ]; then \
		set -a; . ./.env; set +a; go run ./cmd/aide-bot; \
	else \
		go run ./cmd/aide-bot; \
	fi

test:
	go test ./...

check: fmt-check
	go vet ./...
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

fmt-check:
	@test -z "$$(gofmt -l ./cmd ./internal)" || (gofmt -l ./cmd ./internal; exit 1)

build:
	go build -o bin/aide-bot ./cmd/aide-bot
