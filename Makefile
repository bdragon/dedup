SOURCES=$(shell git ls-files | grep -P "(?<!_test)\.go")

.PHONY: test
test:
	go test -cpu=1,2,4,8 -race -vet -v ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: build
build: bin/darwin-amd64/dedup bin/linux-amd64/dedup

bin/darwin-amd64/dedup: $(SOURCES)
	env GOOS=darwin GOARCH=amd64 go build -o bin/darwin-amd64/dedup cmd/dedup/main.go

bin/linux-amd64/dedup: $(SOURCES)
	env GOOS=linux GOARCH=amd64 go build -o bin/linux-amd64/dedup cmd/dedup/main.go

.PHONY: clean
clean:
	rm -rf bin/*
