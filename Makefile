BINARY_NAME=flockman

build:
	go build -o ${BINARY_NAME} main.go

run:
	go build -o ${BINARY_NAME} main.go
	export PORT=8080 && \
	export GIN_MODE=release && \
	./${BINARY_NAME}

clean:
	go clean
