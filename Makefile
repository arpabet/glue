VERSION := $(shell git describe --tags --always --dirty)

all: build

version:
	@echo $(VERSION)

build: version
	go test -cover ./...
	go build -v

update:
	go get -u ./...

bench:
	go test -bench=Benchmark -benchmem -count=1 -run=^$ 2>&1