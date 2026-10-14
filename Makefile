.PHONY: up down wait ingest demo pull-model test-rag test-api

ifneq (,$(wildcard ./.env))
include .env
export
endif

API_BASE_URL ?= http://localhost:8080
RAG_SERVICE_URL ?= http://localhost:8000

up:
	docker compose up -d --build

down:
	docker compose down

wait:
	python scripts/wait_services.py

pull-model:
	docker compose exec ollama ollama pull $(OLLAMA_MODEL)

ingest: wait
	python scripts/seed_corpus.py

demo: up wait pull-model ingest
	@echo "Try: curl -s $(API_BASE_URL)/query -H 'Content-Type: application/json' -d '{\"question\":\"How does ingestion work?\"}'"

test-rag:
	cd services/rag && pytest -q

test-api:
	docker compose up -d db
	docker run --rm --network container:knowledge-assistant-db \
		-v "$(CURDIR):/work" -w /work/services/api \
		golang:1.23-bookworm go test ./...
