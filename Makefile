.PHONY: all fmt server client clean

all: fmt bin_dir server client

fmt:
	@cd server && go fmt ./...
	@cd client && go fmt ./...

bin_dir:
	if [ ! -d "bin" ]; then mkdir bin; fi

server:
	go build -o bin/server server/main.go

client:
	go build -o bin/client client/main.go

clean:
	rm -rf bin/
