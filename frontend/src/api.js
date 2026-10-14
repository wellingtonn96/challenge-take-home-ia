const API_BASE = (import.meta.env.VITE_API_BASE || "http://localhost:8088").replace(
  /\/$/,
  "",
);
const RAG_BASE = (import.meta.env.VITE_RAG_BASE || "http://localhost:8000").replace(
  /\/$/,
  "",
);

export function newTraceId() {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID();
  }
  return `trace-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function traceIdFrom(response, fallback) {
  return response.headers.get("X-Trace-Id") || fallback;
}

async function readJson(response) {
  const text = await response.text();
  let body = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = { message: text };
    }
  }
  if (!response.ok) {
    const message =
      body?.message || body?.detail?.message || response.statusText || "request failed";
    const error = new Error(message);
    error.status = response.status;
    error.body = body;
    throw error;
  }
  return body;
}

export async function fetchHealth() {
  return readJson(await fetch(`${API_BASE}/health`));
}

export async function listDocuments() {
  const data = await readJson(await fetch(`${API_BASE}/documents`));
  return data.documents || [];
}

export async function ingestDocument({ title, text, sourceUri, traceId }) {
  const id = traceId || newTraceId();
  const response = await fetch(`${API_BASE}/ingest`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Trace-Id": id,
    },
    body: JSON.stringify({
      title,
      text,
      source_uri: sourceUri || undefined,
    }),
  });
  const body = await readJson(response);
  return { ...body, traceId: traceIdFrom(response, id) };
}

export async function queryKnowledge({ question, topK, traceId }) {
  const payload = { question };
  if (topK) {
    payload.top_k = Number(topK);
  }
  const id = traceId || newTraceId();
  const response = await fetch(`${API_BASE}/query`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Trace-Id": id,
    },
    body: JSON.stringify(payload),
  });
  const body = await readJson(response);
  return { ...body, traceId: traceIdFrom(response, id) };
}

export async function deleteDocument(id) {
  const response = await fetch(`${API_BASE}/documents/${id}`, { method: "DELETE" });
  if (!response.ok && response.status !== 204) {
    await readJson(response);
  }
}

export async function fetchServiceLogs(baseUrl, serviceName, since) {
  const params = new URLSearchParams({ limit: "120" });
  if (since) {
    params.set("since", since);
  }
  const data = await readJson(await fetch(`${baseUrl}/logs?${params}`));
  return (data.entries || []).map((entry) => ({
    ...entry,
    service: data.service || serviceName,
    trace_id: entry.trace_id || "",
    step: entry.step || "",
  }));
}

export function getApiBase() {
  return API_BASE;
}

export function getRagBase() {
  return RAG_BASE;
}
