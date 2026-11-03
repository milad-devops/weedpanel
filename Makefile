.PHONY: up down logs build backend frontend

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

build:
	docker compose build

backend:
	cd backend && go run ./cmd/api

frontend:
	cd frontend && npm run dev
