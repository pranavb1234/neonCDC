.PHONY: up down logs reset

up:
	docker compose up -d redis

down:
	docker compose down

logs:
	docker compose logs -f redis

reset:
	docker compose down -v
	docker compose up -d redis
