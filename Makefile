COMPOSE = docker compose --env-file .env -f deploy/docker-compose.yml

.PHONY: up down build rebuild logs ps

up:
	$(COMPOSE) up

down:
	$(COMPOSE) down

build:
	$(COMPOSE) build

rebuild:
	$(COMPOSE) up --build

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps
