BINARY_NAME=flockman
LDFLAGS=-ldflags "-s -w -X github.com/erfantkerfan/flockman/cmd.version=0.0.0"

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) main.go

run: build
	PORT=8314 GIN_MODE=release ./$(BINARY_NAME)

serve: build
	PORT=8314 GIN_MODE=release ./$(BINARY_NAME) serve

clean:
	go clean
	rm -f $(BINARY_NAME)

.PHONY: build run serve clean
