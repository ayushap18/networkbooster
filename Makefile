.PHONY: cli gui all test clean

cli:
	go build -o bin/networkbooster ./cmd/cli

gui:
	@echo "GUI build not yet implemented"

all: cli

test:
	go test ./... -v

clean:
	rm -rf bin/
