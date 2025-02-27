BINARY_NAME=flockman

build:
	go build -ldflags "-s -w -X github.com/erfantkerfan/flockman/cmd.version=0.0.0" -o ${BINARY_NAME} main.go

run:
	go build -ldflags "-s -w -X github.com/erfantkerfan/flockman/cmd.version=0.0.0" -o ${BINARY_NAME} main.go
	export PORT=8314 && \
	export GIN_MODE=release && \
	./${BINARY_NAME}

serve:
	go build -ldflags "-s -w -X github.com/erfantkerfan/flockman/cmd.version=0.0.0" -o ${BINARY_NAME} main.go
	export PORT=8314 && \
	export GIN_MODE=release && \
	./${BINARY_NAME} serve

clean:
	go clean
