.PHONY: build clean
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/mesh-server ./cmd/mesh-server
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/mesh-agent ./cmd/mesh-agent
clean:
	rm -rf bin
