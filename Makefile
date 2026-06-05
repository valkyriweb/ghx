all: build

build:
	go build -o bin/ghx ./src/cmd/ghx
	go build -o bin/ghxd ./src/cmd/ghxd

test:
	go test ./...

verify:
	./scripts/verify

release:
	@test -n "$(VERSION)" || { echo "usage: make release VERSION=vX.Y.Z"; exit 1; }
	./scripts/release $(VERSION)

clean:
	rm -rf bin/ dist/

.PHONY: all build test verify release clean
