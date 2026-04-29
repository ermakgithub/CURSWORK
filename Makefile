APP_NAME=logistics-routing

.PHONY: up down build logs local tidy

up:
	docker compose up --build -d

down:
	docker compose down

build:
	docker compose build

logs:
	docker compose logs -f

local:
	go run main.go

tidy:
	go mod tidy
