const STEP_RANK = {
  start: 10,
  "db.insert": 20,
  "rag.ingest": 30,
  chunk: 40,
  "embed.batch": 50,
  "ingest.ok": 60,
  "rag.query": 30,
  embed: 40,
  search: 50,
  prompt: 60,
  llm: 70,
  "llm.ok": 80,
  "query.ok": 90,
  done: 100,
};

export function shortTraceId(traceId) {
  if (!traceId) return "—";
  return traceId.length > 8 ? traceId.slice(0, 8) : traceId;
}

export function compareFlowEntries(a, b) {
  const ta = String(a.time).localeCompare(String(b.time));
  if (ta !== 0) return ta;
  const ra = STEP_RANK[a.step] ?? 50;
  const rb = STEP_RANK[b.step] ?? 50;
  if (ra !== rb) return ra - rb;
  return String(a.service).localeCompare(String(b.service));
}

export function groupLogsByTrace(logs) {
  const groups = new Map();
  for (const line of logs) {
    const key = line.trace_id || "";
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key).push(line);
  }
  for (const items of groups.values()) {
    items.sort(compareFlowEntries);
  }
  const sorted = [...groups.entries()].sort((a, b) => {
    const lastA = a[1][a[1].length - 1]?.time || "";
    const lastB = b[1][b[1].length - 1]?.time || "";
    return String(lastB).localeCompare(String(lastA));
  });
  return sorted;
}

export function logEntryKey(line) {
  return `${line.service}|${line.time}|${line.step}|${line.trace_id}|${line.message}`;
}
