MODULE := github.com/koopa0/kurodo

.PHONY: build run test lint fmt vet gen css verify clean

build: gen css
	go build -o bin/kurodo ./cmd/kurodo

run: gen css
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
	go tool templ generate

css:
	tailwindcss -i assets/css/input.css -o assets/css/output.css --minify

verify: fmt vet lint test build

clean:
	rm -rf bin tmp
