.PHONY: build test vet lint install clean

BINARY := devrec
LDFLAGS := -s -w

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

test:
	go test -race -cover ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

install: build
	sudo install -m 755 $(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY) devrec-linux-*
