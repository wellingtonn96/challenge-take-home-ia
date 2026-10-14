FROM python:3.11-slim-bookworm

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential curl \
    && rm -rf /var/lib/apt/lists/*

COPY pyproject.toml README.md ./
COPY rag ./rag

# CPU-only PyTorch keeps the image smaller than the default CUDA stack from pip.
RUN pip install torch --index-url https://download.pytorch.org/whl/cpu \
    && pip install -e .

EXPOSE 8000

HEALTHCHECK --interval=10s --timeout=5s --retries=12 --start-period=120s \
    CMD curl -fsS http://127.0.0.1:8000/health || exit 1

CMD ["uvicorn", "rag.app:app", "--host", "0.0.0.0", "--port", "8000"]
