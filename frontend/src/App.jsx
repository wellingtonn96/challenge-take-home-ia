import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  deleteDocument,
  fetchHealth,
  fetchServiceLogs,
  getApiBase,
  getRagBase,
  ingestDocument,
  listDocuments,
  queryKnowledge,
} from "./api.js";
import {
  compareFlowEntries,
  groupLogsByTrace,
  logEntryKey,
  shortTraceId,
} from "./flowLogs.js";

function formatTime(iso) {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleTimeString();
}

export default function App() {
  const [health, setHealth] = useState(null);
  const [documents, setDocuments] = useState([]);
  const [title, setTitle] = useState("Product FAQ");
  const [text, setText] = useState("");
  const [question, setQuestion] = useState("What is the default embedding model?");
  const [topK, setTopK] = useState("5");
  const [answer, setAnswer] = useState("");
  const [sources, setSources] = useState([]);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [logs, setLogs] = useState([]);
  const [logPaused, setLogPaused] = useState(false);
  const [logMode, setLogMode] = useState("debugger");
  const [focusTraceId, setFocusTraceId] = useState("");
  const [traceOnly, setTraceOnly] = useState(true);
  const lastLogAt = useRef("");

  const refreshDocuments = useCallback(async () => {
    const items = await listDocuments();
    setDocuments(items);
  }, []);

  const refreshHealth = useCallback(async () => {
    try {
      setHealth(await fetchHealth());
    } catch {
      setHealth({ status: "down" });
    }
  }, []);

  const pollLogs = useCallback(async () => {
    if (logPaused) return;
    const since = lastLogAt.current || undefined;
    const [apiEntries, ragEntries] = await Promise.all([
      fetchServiceLogs(getApiBase(), "api", since).catch(() => []),
      fetchServiceLogs(getRagBase(), "rag", since).catch(() => []),
    ]);
    const merged = [...apiEntries, ...ragEntries].sort(compareFlowEntries);
    if (merged.length === 0) return;
    setLogs((prev) => {
      const seen = new Set(prev.map(logEntryKey));
      const next = [...prev];
      for (const entry of merged) {
        const key = logEntryKey(entry);
        if (!seen.has(key)) {
          seen.add(key);
          next.push(entry);
        }
      }
      return next.slice(-400);
    });
    const latest = merged[merged.length - 1]?.time;
    if (latest) {
      lastLogAt.current = latest;
    }
  }, [logPaused]);

  useEffect(() => {
    refreshHealth();
    refreshDocuments();
  }, [refreshHealth, refreshDocuments]);

  useEffect(() => {
    const intervalMs = busy || focusTraceId ? 800 : 2000;
    const timer = setInterval(() => {
      pollLogs();
    }, intervalMs);
    pollLogs();
    return () => clearInterval(timer);
  }, [pollLogs, busy, focusTraceId]);

  async function onIngest(event) {
    event.preventDefault();
    setBusy("ingest");
    setError("");
    try {
      const result = await ingestDocument({ title, text });
      if (result.traceId) {
        setFocusTraceId(result.traceId);
        setTraceOnly(true);
      }
      setAnswer("");
      setSources([]);
      await refreshDocuments();
      setError(`Ingest OK — ${result.chunks_created} chunks (id ${result.document_id})`);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy("");
    }
  }

  async function onQuery(event) {
    event.preventDefault();
    setBusy("query");
    setError("");
    try {
      const result = await queryKnowledge({ question, topK });
      if (result.traceId) {
        setFocusTraceId(result.traceId);
        setTraceOnly(true);
      }
      setAnswer(result.answer || "");
      setSources(result.sources || []);
    } catch (err) {
      setAnswer("");
      setSources([]);
      setError(err.message);
    } finally {
      setBusy("");
    }
  }

  async function onDelete(id) {
    setBusy("delete");
    setError("");
    try {
      await deleteDocument(id);
      await refreshDocuments();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy("");
    }
  }

  const logLines = useMemo(() => {
    if (!traceOnly || !focusTraceId) return logs;
    return logs.filter((line) => line.trace_id === focusTraceId);
  }, [logs, traceOnly, focusTraceId]);

  const traceGroups = useMemo(() => groupLogsByTrace(logLines), [logLines]);

  return (
    <div className="layout">
      <header className="header">
        <div>
          <h1>Knowledge Assistant</h1>
          <p className="muted">
            Cliente React → API Go ({getApiBase()}) → RAG Python ({getRagBase()})
          </p>
        </div>
        <div className="pill">
          API: {health?.status === "ok" ? "online" : "offline"}
        </div>
      </header>

      <div className="grid">
        <section className="panel">
          <h2>Corpus</h2>
          <form className="stack" onSubmit={onIngest}>
            <label>
              Título
              <input value={title} onChange={(e) => setTitle(e.target.value)} required />
            </label>
            <label>
              Texto
              <textarea
                rows={6}
                value={text}
                onChange={(e) => setText(e.target.value)}
                placeholder="Cole o conteúdo do documento..."
                required
              />
            </label>
            <button type="submit" disabled={busy === "ingest"}>
              {busy === "ingest" ? "Enviando…" : "Ingerir via API Go"}
            </button>
          </form>

          <h3>Documentos</h3>
          <ul className="doc-list">
            {documents.map((doc) => (
              <li key={doc.id}>
                <div>
                  <strong>{doc.title}</strong>
                  <span className="muted">{doc.id}</span>
                </div>
                <button type="button" onClick={() => onDelete(doc.id)} disabled={!!busy}>
                  Remover
                </button>
              </li>
            ))}
            {documents.length === 0 && <li className="muted">Nenhum documento.</li>}
          </ul>

          <h2>Pergunta</h2>
          <form className="stack" onSubmit={onQuery}>
            <label>
              Pergunta
              <input value={question} onChange={(e) => setQuestion(e.target.value)} required />
            </label>
            <label>
              top_k
              <input
                type="number"
                min={1}
                max={50}
                value={topK}
                onChange={(e) => setTopK(e.target.value)}
              />
            </label>
            <button type="submit" disabled={busy === "query"}>
              {busy === "query" ? "Consultando…" : "Perguntar via API Go"}
            </button>
          </form>

          {error && <p className="banner">{error}</p>}

          {answer && (
            <div className="answer">
              <h3>Resposta</h3>
              <p>{answer}</p>
              {sources.length > 0 && (
                <>
                  <h4>Fontes</h4>
                  <ul>
                    {sources.map((src) => (
                      <li key={src.chunk_id}>
                        <span className="muted">score {src.score?.toFixed?.(3) ?? src.score}</span>
                        {src.content}
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </div>
          )}
        </section>

        <section className="panel logs">
          <div className="logs-header">
            <h2>Debugger de fluxo</h2>
            <div className="logs-actions">
              <button
                type="button"
                className={logMode === "debugger" ? "active" : ""}
                onClick={() => setLogMode("debugger")}
              >
                Fluxo
              </button>
              <button
                type="button"
                className={logMode === "stream" ? "active" : ""}
                onClick={() => setLogMode("stream")}
              >
                Stream
              </button>
              <button type="button" onClick={() => setLogPaused((v) => !v)}>
                {logPaused ? "Retomar" : "Pausar"}
              </button>
              <button
                type="button"
                onClick={() => {
                  setLogs([]);
                  lastLogAt.current = "";
                  setFocusTraceId("");
                }}
              >
                Limpar
              </button>
            </div>
          </div>
          <p className="muted">
            Cada ação envia <code>X-Trace-Id</code> da UI → API Go → RAG Python. Passos nomeados
            (<code>embed</code>, <code>search</code>, <code>llm</code>…) aparecem agrupados como um
            trace distribuído (só dev, buffer em memória).
          </p>
          {focusTraceId && (
            <div className="trace-focus">
              <span>
                Trace ativo: <code>{shortTraceId(focusTraceId)}</code>
              </span>
              <label className="trace-only">
                <input
                  type="checkbox"
                  checked={traceOnly}
                  onChange={(e) => setTraceOnly(e.target.checked)}
                />
                Só este fluxo
              </label>
              <button type="button" onClick={() => setFocusTraceId("")}>
                Mostrar todos
              </button>
            </div>
          )}
          <div className="log-view" aria-live="polite">
            {logMode === "stream" &&
              logLines.map((line) => (
                <div key={logEntryKey(line)} className={`log log-${line.level?.toLowerCase()}`}>
                  <span className="log-meta">
                    {formatTime(line.time)} [{line.service}] {line.level}
                    {line.step ? ` · ${line.step}` : ""}
                    {line.trace_id ? ` · ${shortTraceId(line.trace_id)}` : ""}
                  </span>
                  <span>{line.message}</span>
                </div>
              ))}
            {logMode === "debugger" &&
              traceGroups.map(([traceId, steps]) => (
                <details
                  key={traceId || "no-trace"}
                  className="trace-group"
                  open={!traceId || traceId === focusTraceId || traceGroups.length === 1}
                >
                  <summary>
                    <span className="trace-id">
                      {traceId ? shortTraceId(traceId) : "sem trace"}
                    </span>
                    <span className="muted">
                      {steps[0]?.message?.slice(0, 48) || "—"} · {steps.length} eventos
                    </span>
                    {traceId && (
                      <button
                        type="button"
                        className="trace-pin"
                        onClick={(e) => {
                          e.preventDefault();
                          setFocusTraceId(traceId);
                          setTraceOnly(true);
                        }}
                      >
                        Focar
                      </button>
                    )}
                  </summary>
                  <ol className="trace-steps">
                    {steps.map((line) => (
                      <li
                        key={logEntryKey(line)}
                        className={`trace-step log-${line.level?.toLowerCase()}`}
                      >
                        <span className="step-badge">{line.step || "log"}</span>
                        <span className="step-service">{line.service}</span>
                        <span className="step-time">{formatTime(line.time)}</span>
                        <span className="step-msg">{line.message}</span>
                      </li>
                    ))}
                  </ol>
                </details>
              ))}
            {logLines.length === 0 && <p className="muted">Aguardando logs…</p>}
          </div>
        </section>
      </div>
    </div>
  );
}
