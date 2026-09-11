GO ?= go

.PHONY: build test vet race fmt generate check-generated lint check-state-imports demo
build:
	$(GO) build -o bin/raftkv ./cmd/raftkv
	$(GO) build -o bin/raftctl ./cmd/raftctl
test: check-state-imports
	$(GO) test ./...
vet:
	$(GO) vet ./...
race:
	$(GO) test -race ./...
fmt:
	gofmt -w cmd internal
generate:
	sh scripts/generate-proto.sh
check-generated: generate
	git diff --exit-code -- proto/raftkvpb
	test -z "$$(git ls-files --others --exclude-standard -- proto/raftkvpb)"
lint:
	golangci-lint run
check-state-imports:
	GO=$(GO) sh scripts/check-state-imports.sh
demo:
	$(GO) run ./cmd/raftkv --demo --http-addr 127.0.0.1:8080 --grpc-addr 127.0.0.1:9090
