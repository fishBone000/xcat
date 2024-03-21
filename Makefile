VERSION = $(shell git describe --tags)

dev:
	go build -v -ldflags="-X main.version=$(VERSION)-dev" -o build/xcat

release:
	go build -v -ldflags="-X main.version=$(VERSION)" -o build/xcat
