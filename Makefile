MODULE := github.com/koopa0/kurodo

.PHONY: build run test lint fmt vet gen css sqlc verify verify-spec clean

build: gen
	go build -o bin/kurodo ./cmd/kurodo

run: gen
	go run ./cmd/kurodo serve

test:
	go test -race -count=1 -shuffle=on ./...

lint:
	golangci-lint config verify
	golangci-lint run

fmt:
	goimports -w -local $(MODULE) .

vet:
	go vet ./...

gen:
	templ generate

css:
	tailwindcss -i assets/css/input.css -o assets/css/output.css --minify

sqlc:
	sqlc generate

verify: fmt vet lint test build

clean:
	rm -rf bin tmp

verify-spec:
	@echo "=== Hook Tests ==="
	@bash tests/test-hooks.sh
	@echo ""
	@echo "=== Skill/Agent Format Tests ==="
	@bash tests/test-skill-format.sh
	@echo ""
	@echo "=== Consistency Tests ==="
	@bash tests/test-consistency.sh
