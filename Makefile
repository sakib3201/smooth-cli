.PHONY: test test-race test-integration lint build clean

test:
	go test ./...

test-race:
	go test -race -count=1 ./...

test-integration:
	go test -tags=integration -timeout=120s ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

build:
	go build -o bin/smooth ./cmd/smooth

build-all:
	GOOS=darwin  GOARCH=amd64  go build -o bin/smooth-darwin-amd64  ./cmd/smooth
	GOOS=darwin  GOARCH=arm64  go build -o bin/smooth-darwin-arm64   ./cmd/smooth
	GOOS=linux   GOARCH=amd64  go build -o bin/smooth-linux-amd64   ./cmd/smooth
	GOOS=windows GOARCH=amd64  go build -o bin/smooth-windows-amd64.exe ./cmd/smooth

clean:
	rm -rf bin/ coverage.out coverage.html

vet:
	go vet ./...

corpus-check:
	go test -run TestAttentionCorpus ./internal/attention/... -v
