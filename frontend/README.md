# Frontend (React)

Painel simples em duas colunas:

- **Esquerda:** ingest, lista de documentos, pergunta (tudo via **API Go**).
- **Direita:** logs recentes da API Go e do RAG Python (`GET /logs` em cada serviço).

## Dev (host)

```bash
cd frontend
cp .env.example .env
npm install
npm run dev
```

Abra `http://localhost:5173`. Ajuste `VITE_API_BASE` e `VITE_RAG_BASE` no `.env` se suas portas forem diferentes.

Requisitos:

- API Go com CORS (`CORS_ALLOWED_ORIGINS` inclui `http://localhost:5173`).
- RAG Python rodando com o mesmo CORS.

## Docker

Na raiz do repo (usa `API_PORT` do `.env`):

```bash
docker compose up -d --build web
```

Abra `http://localhost:5173`.

**Limitação:** o buffer de logs é em memória e some ao reiniciar o processo. Para produção use agregador (Loki, CloudWatch, etc.).
