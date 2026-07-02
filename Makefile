MODULE := github.com/koopa0/yomihon

.PHONY: build run test lint fmt vet gen css sqlc verify clean

build: gen
	go build -o bin/yomihon ./cmd/yomihon

run: gen
	go run ./cmd/yomihon serve

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
