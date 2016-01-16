export GOOS = linux

all:
	@echo "Compiling Go binaries"
	@go build -o bin/agentd ./cmd/agentd
	@go build -o bin/supervisord ./cmd/supervisord
	@docker-compose build --no-cache --force-rm

up: all
	@docker-compose up
