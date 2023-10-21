VERSION = $(shell git describe --tags)
build:
	go build -v -ldflags="-X main.version=$(VERSION)" -o build/xcat_"$(VERSION)"
