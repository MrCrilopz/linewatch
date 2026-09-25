import { useEffect, useRef, useState } from "react";
import es from "./i18n/es.json";
import en from "./i18n/en.json";

const copy = { es, en };
const steps = ["stepBaseline", "stepDetect", "stepCorrelate", "stepEvents", "stepExplain"];

function text(lang, key) {
  return copy[lang][key] || key;
}

async function api(path, token, lang, options = {}) {
  const headers = { "Accept-Language": lang, ...(options.headers || {}) };
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(path, { ...options, headers });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || "error");
  return body;
}

export default function App() {
  const [lang, setLang] = useState(() => localStorage.getItem("linewatch_lang") || "es");
  const [token, setToken] = useState(() => sessionStorage.getItem("linewatch_token") || "");
  const [view, setView] = useState("dashboard");
  const [meterId, setMeterId] = useState("");
  const [anomalyId, setAnomalyId] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loginError, setLoginError] = useState(false);
  const [summary, setSummary] = useState(null);
  const [running, setRunning] = useState(false);
  const [step, setStep] = useState(0);
  const [outcome, setOutcome] = useState(null);

  useEffect(() => {
    localStorage.setItem("linewatch_lang", lang);
    document.documentElement.lang = lang;
  }, [lang]);

  useEffect(() => {
    if (!token) return;
    api("/dashboard/summary", token, lang).then(setSummary).catch(() => {
      sessionStorage.removeItem("linewatch_token");
      setToken("");
    });
  }, [token, lang]);

  useEffect(() => {
    if (!running) return;
    const id = setInterval(() => setStep((n) => (n + 1) % steps.length), 700);
    return () => clearInterval(id);
  }, [running]);

  async function onLogin(event) {
    event.preventDefault();
    setLoginError(false);
    try {
      const body = await api("/login", "", lang, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      sessionStorage.setItem("linewatch_token", body.token);
      setToken(body.token);
      setPassword("");
    } catch {
      setLoginError(true);
    }
  }

  function logout() {
    sessionStorage.removeItem("linewatch_token");
    setToken("");
    setSummary(null);
    setOutcome(null);
  }

  async function runAnalysis() {
    setRunning(true);
    setStep(0);
    setOutcome(null);
    try {
      const started = await api("/ai/analyze", token, lang, { method: "POST" });
      let row = started;
      for (let i = 0; i < 40 && row.status === "running"; i += 1) {
        await new Promise((resolve) => setTimeout(resolve, 400));
        row = await api(`/ai/analysis/${started.id}`, token, lang);
      }
      const next = await api("/dashboard/summary", token, lang);
      setSummary(next);
      setOutcome({ n: next.anomaly_count, h: next.high_priority_count, failed: false });
    } catch {
      setOutcome({ failed: true });
    } finally {
      setRunning(false);
    }
  }

  if (!token) {
    return (
      <main className="login">
        <form onSubmit={onLogin}>
          <p className="brand">{text(lang, "product")}</p>
          <h1>{text(lang, "loginTitle")}</h1>
          <label>
            {text(lang, "username")}
            <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
          </label>
          <label>
            {text(lang, "password")}
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" />
          </label>
          {loginError ? <p className="error">{text(lang, "loginError")}</p> : null}
          <button type="submit">{text(lang, "signIn")}</button>
          <Language lang={lang} setLang={setLang} />
        </form>
      </main>
    );
  }

  const status = summary?.analysis_status || "none";
  const statusKey = `status${status[0].toUpperCase()}${status.slice(1)}`;
  const result = !outcome
    ? ""
    : outcome.failed
      ? text(lang, "statusError")
      : outcome.n === 4 && outcome.h === 2
        ? text(lang, "priority")
        : text(lang, "result").replace("{{n}}", String(outcome.n)).replace("{{h}}", String(outcome.h));
  const when = summary?.last_analysis_at
    ? new Date(summary.last_analysis_at).toLocaleString(lang === "es" ? "es" : "en")
    : text(lang, "statusNone");

  return (
    <div className="shell">
      <header>
        <strong>{text(lang, "product")}</strong>
        <nav>
          <button type="button" className={view === "dashboard" ? "on" : ""} onClick={() => setView("dashboard")}>
            {text(lang, "dashboard")}
          </button>
          <button type="button" className={view === "meters" ? "on" : ""} onClick={() => { setMeterId(""); setView("meters"); }}>
            {text(lang, "meters")}
          </button>
          <button type="button" className={view === "anomalies" ? "on" : ""} onClick={() => { setAnomalyId(""); setView("anomalies"); }}>
            {text(lang, "anomalies")}
          </button>
        </nav>
        <div className="tools">
          <Language lang={lang} setLang={setLang} />
          <button type="button" onClick={logout}>{text(lang, "logout")}</button>
        </div>
      </header>
      {view === "meters" ? (
        meterId ? (
          <MeterDetail
            lang={lang}
            token={token}
            meterId={meterId}
            onBack={() => setMeterId("")}
            running={running}
            step={step}
            onRun={runAnalysis}
            result={result}
            onInvestigate={(id) => { setAnomalyId(id); setView("anomalies"); }}
          />
        ) : (
          <MeterList lang={lang} token={token} onOpen={setMeterId} />
        )
      ) : view === "anomalies" ? (
        anomalyId ? (
          <AnomalyDetail lang={lang} token={token} anomalyId={anomalyId} onBack={() => setAnomalyId("")} />
        ) : (
          <AnomalyList lang={lang} token={token} onOpen={setAnomalyId} />
        )
      ) : (
        <section className="panel">
          <div className="kpis">
            <article><span>{text(lang, "kMeters")}</span><strong>{summary ? summary.meter_count : "—"}</strong></article>
            <article><span>{text(lang, "kConsumption")}</span><strong>{summary ? Math.round(summary.period_consumption_kwh).toLocaleString(lang) : "—"} kWh</strong></article>
            <article><span>{text(lang, "kAnomalies")}</span><strong>{summary ? summary.anomaly_count : "—"}</strong></article>
            <article><span>{text(lang, "kHigh")}</span><strong>{summary ? summary.high_priority_count : "—"}</strong></article>
            <article><span>{text(lang, "kConfidence")}</span><strong>{summary ? summary.confidence.toFixed(2) : "—"}</strong></article>
            <article><span>{text(lang, "kLast")}</span><strong>{text(lang, statusKey)}</strong><small>{when}</small></article>
          </div>
          <button type="button" className="run" onClick={runAnalysis} disabled={running}>
            {running ? text(lang, steps[step]) : text(lang, "run")}
          </button>
          {result ? <p className="result">{result}</p> : null}
        </section>
      )}
    </div>
  );
}

function MeterList({ lang, token, onOpen }) {
  const [status, setStatus] = useState("all");
  const [q, setQ] = useState("");
  const [sort, setSort] = useState("consumption");
  const [meters, setMeters] = useState([]);
  const [anomalies, setAnomalies] = useState({});
  const locale = lang === "es" ? "es" : "en";

  useEffect(() => {
    const params = new URLSearchParams({ status, q, sort });
    api(`/meters?${params}`, token, lang).then((body) => setMeters(body.meters || [])).catch(() => setMeters([]));
  }, [token, lang, status, q, sort]);

  useEffect(() => {
    api("/anomalies", token, lang)
      .then((body) => {
        const map = {};
        for (const item of body.anomalies || []) map[item.meter_id] = item;
        setAnomalies(map);
      })
      .catch(() => setAnomalies({}));
  }, [token, lang]);

  return (
    <section className="panel">
      <h1>{text(lang, "meters")}</h1>
      <div className="filters">
        <select value={status} onChange={(e) => setStatus(e.target.value)} aria-label={text(lang, "colStatus")}>
          <option value="all">{text(lang, "filterAll")}</option>
          <option value="ok">{text(lang, "filterOk")}</option>
          <option value="alert">{text(lang, "filterAlert")}</option>
          <option value="critical">{text(lang, "filterCritical")}</option>
        </select>
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder={text(lang, "search")} aria-label={text(lang, "search")} />
        <select value={sort} onChange={(e) => setSort(e.target.value)} aria-label={text(lang, "sortConsumption")}>
          <option value="consumption">{text(lang, "sortConsumption")}</option>
          <option value="variation">{text(lang, "sortVariation")}</option>
          <option value="severity">{text(lang, "sortSeverity")}</option>
        </select>
      </div>
      <table>
        <thead>
          <tr>
            <th>{text(lang, "colMeter")}</th>
            <th>{text(lang, "colConsumption")}</th>
            <th>{text(lang, "colVariation")}</th>
            <th>{text(lang, "colStatus")}</th>
            <th>{text(lang, "colAnomaly")}</th>
          </tr>
        </thead>
        <tbody>
          {meters.length === 0 ? (
            <tr><td colSpan="5">{text(lang, "empty")}</td></tr>
          ) : meters.map((meter) => (
            <tr key={meter.meter_id} onClick={() => onOpen(meter.meter_id)}>
              <td>{meter.meter_id}</td>
              <td>{kwh(meter.consumption_kwh, locale)}</td>
              <td>{percent(meter.variation, locale)}</td>
              <td>{text(lang, statusKey(meter.status))}</td>
              <td>{anomalies[meter.meter_id]?.type || "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function MeterDetail({ lang, token, meterId, onBack, running, step, onRun, result, onInvestigate }) {
  const [meter, setMeter] = useState(null);
  const [readings, setReadings] = useState([]);
  const [caseId, setCaseId] = useState("");
  const locale = lang === "es" ? "es" : "en";

  useEffect(() => {
    api(`/meters/${meterId}`, token, lang).then(setMeter).catch(() => setMeter(null));
    api(`/meters/${meterId}/readings`, token, lang).then((body) => setReadings(body.readings || [])).catch(() => setReadings([]));
    api("/anomalies", token, lang)
      .then((body) => {
        const hit = (body.anomalies || []).find((item) => item.meter_id === meterId);
        setCaseId(hit ? hit.id : "");
      })
      .catch(() => setCaseId(""));
  }, [token, lang, meterId]);

  const last = readings[readings.length - 1];
  const hourlyBaseline = meter && readings.length ? meter.baseline_kwh / readings.length : 0;

  return (
    <section className="panel">
      <button type="button" className="back" onClick={onBack}>{text(lang, "back")}</button>
      <h1>{meterId}</h1>
      {caseId ? <button type="button" onClick={() => onInvestigate(caseId)}>{text(lang, "investigate")}</button> : null}
      {meter ? (
        <div className="kpis">
          <article><span>{text(lang, "colConsumption")}</span><strong>{kwh(meter.consumption_kwh, locale)}</strong></article>
          <article><span>{text(lang, "baseline")}</span><strong>{kwh(meter.baseline_kwh, locale)}</strong></article>
          <article><span>{text(lang, "colVariation")}</span><strong>{percent(meter.variation, locale)}</strong></article>
          <article><span>{text(lang, "colStatus")}</span><strong>{text(lang, statusKey(meter.status))}</strong></article>
          <article><span>{text(lang, "voltage")}</span><strong>{last ? `${last.voltage_v.toFixed(1)} V` : "—"}</strong></article>
          <article><span>{text(lang, "current")}</span><strong>{last ? `${last.current_a.toFixed(1)} A` : "—"}</strong></article>
          <article><span>{text(lang, "powerFactor")}</span><strong>{last ? last.power_factor.toFixed(2) : "—"}</strong></article>
        </div>
      ) : null}
      <h2>{text(lang, "history")}</h2>
      <LineChart
        values={readings.map((row) => row.consumption_kwh)}
        times={readings.map((row) => row.timestamp)}
        baseline={hourlyBaseline}
        label={text(lang, "history")}
        format={(value) => `${value.toLocaleString(locale, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} kWh`}
        note={text(lang, "baseline")}
        locale={locale}
      />
      <h2>{text(lang, "electrical")}</h2>
      <LineChart
        values={readings.map((row) => row.current_a)}
        times={readings.map((row) => row.timestamp)}
        baseline={0}
        label={text(lang, "current")}
        format={(value) => `${value.toLocaleString(locale, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} A`}
        locale={locale}
      />
      <button type="button" className="run" onClick={onRun} disabled={running}>
        {running ? text(lang, steps[step]) : text(lang, "run")}
      </button>
      {result ? <p className="result">{result}</p> : null}
    </section>
  );
}

function AnomalyList({ lang, token, onOpen }) {
  const [items, setItems] = useState([]);
  const [ready, setReady] = useState(false);
  const locale = lang === "es" ? "es" : "en";

  useEffect(() => {
    api("/anomalies", token, lang)
      .then((body) => setItems(body.anomalies || []))
      .catch(() => setItems([]))
      .finally(() => setReady(true));
  }, [token, lang]);

  return (
    <section className="panel">
      <h1>{text(lang, "anomalies")}</h1>
      <table>
        <thead>
          <tr>
            <th>{text(lang, "colMeter")}</th>
            <th>{text(lang, "colType")}</th>
            <th>{text(lang, "colSeverity")}</th>
            <th>{text(lang, "colConfidence")}</th>
            <th>{text(lang, "colAction")}</th>
          </tr>
        </thead>
        <tbody>
          {items.length === 0 ? (
            <tr><td colSpan="5">{ready ? text(lang, "noAnomalies") : "—"}</td></tr>
          ) : items.map((item) => (
            <tr key={item.id} onClick={() => onOpen(item.id)}>
              <td>{item.meter_id}</td>
              <td>{text(lang, `type${item.type}`)}</td>
              <td>{text(lang, `sev${item.severity}`)}</td>
              <td>{item.confidence.toLocaleString(locale, { style: "percent", maximumFractionDigits: 0 })}</td>
              <td>{item.recommended_action}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function AnomalyDetail({ lang, token, anomalyId, onBack }) {
  const [item, setItem] = useState(null);
  const locale = lang === "es" ? "es" : "en";

  useEffect(() => {
    api(`/anomalies/${anomalyId}`, token, lang).then(setItem).catch(() => setItem(null));
  }, [token, lang, anomalyId]);

  if (!item) {
    return (
      <section className="panel">
        <button type="button" className="back" onClick={onBack}>{text(lang, "back")}</button>
      </section>
    );
  }

  const event = item.event_type
    ? `${item.event_type}${item.event_description ? ` — ${item.event_description}` : ""}`
    : text(lang, "noEvent");
  const signals = (item.signals || []).map((signal) => text(lang, signal));

  return (
    <section className="panel">
      <button type="button" className="back" onClick={onBack}>{text(lang, "back")}</button>
      <h1>{item.meter_id}</h1>
      <dl className="case">
        <dt>{text(lang, "found")}</dt>
        <dd>{item.reason}</dd>
        <dt>{text(lang, "variables")}</dt>
        <dd>
          {signals.join(" · ") || "—"}
          {item.voltage_v ? ` · ${item.voltage_v.toFixed(1)} V` : ""}
          {item.current_a ? ` · ${item.current_a.toFixed(1)} A` : ""}
          {item.power_factor ? ` · ${text(lang, "powerFactor")} ${item.power_factor.toFixed(2)}` : ""}
        </dd>
        <dt>{text(lang, "compare")}</dt>
        <dd>{text(lang, "baseline")} {kwh(item.baseline_kwh, locale)} · {text(lang, "actual")} {kwh(item.actual_kwh, locale)} · {percent(item.variation_pct / 100, locale)}</dd>
        <dt>{text(lang, "events")}</dt>
        <dd>{event}</dd>
        <dt>{text(lang, "sevConf")}</dt>
        <dd>{text(lang, `sev${item.severity}`)} · {item.confidence.toLocaleString(locale, { style: "percent", maximumFractionDigits: 0 })}</dd>
        <dt>{text(lang, "action")}</dt>
        <dd>{item.recommended_action}</dd>
        <dt>{text(lang, "evidence")}</dt>
        <dd>{signals.join(" · ")}</dd>
      </dl>
    </section>
  );
}

function LineChart({ values, times, baseline, label, format, note, locale }) {
  const guide = useRef(null);
  const dot = useRef(null);
  const tip = useRef(null);
  if (!values.length) return null;
  const w = 640;
  const h = 160;
  const pad = 8;
  const max = Math.max(...values, baseline || 0, 1);
  const x = (i) => pad + (i * (w - pad * 2)) / Math.max(values.length - 1, 1);
  const y = (v) => h - pad - (v / max) * (h - pad * 2);
  const d = values.map((v, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");

  function hover(event) {
    const rect = event.currentTarget.getBoundingClientRect();
    const px = ((event.clientX - rect.left) / rect.width) * w;
    const ratio = (px - pad) / (w - pad * 2);
    const next = Math.max(0, Math.min(values.length - 1, Math.round(ratio * (values.length - 1))));
    const at = Math.max(pad, Math.min(w - pad, px));
    guide.current.setAttribute("x1", at);
    guide.current.setAttribute("x2", at);
    guide.current.style.display = "";
    dot.current.setAttribute("cx", x(next));
    dot.current.setAttribute("cy", y(values[next]));
    dot.current.style.display = "";
    const when = new Date(times[next]).toLocaleString(locale, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
    tip.current.textContent = `${when} · ${format(values[next])}`;
  }

  function leave() {
    guide.current.style.display = "none";
    dot.current.style.display = "none";
    tip.current.textContent = "";
  }

  return (
    <div className="chart-wrap">
      {baseline > 0 ? <p className="chart-key"><i />{note} · {format(baseline)}</p> : null}
      <svg className="chart" viewBox={`0 0 ${w} ${h}`} role="img" aria-label={label} onMouseMove={hover} onMouseLeave={leave}>
        {baseline > 0 ? <line className="base" x1={pad} x2={w - pad} y1={y(baseline)} y2={y(baseline)} /> : null}
        <path d={d} />
        <line ref={guide} className="guide" y1={pad} y2={h - pad} style={{ display: "none" }} />
        <circle ref={dot} r="4" style={{ display: "none" }} />
      </svg>
      <p className="chart-tip" ref={tip} />
    </div>
  );
}

function kwh(value, locale) {
  return `${Math.round(value).toLocaleString(locale)} kWh`;
}

function percent(value, locale) {
  const n = value * 100;
  const body = Math.abs(n).toLocaleString(locale, { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  return `${n > 0 ? "+" : n < 0 ? "−" : ""}${body}%`;
}

function statusKey(status) {
  if (status === "alert") return "statusAlert";
  if (status === "critical") return "statusCritical";
  return "statusOk";
}

function Language({ lang, setLang }) {
  return (
    <label className="lang">
      {text(lang, "language")}
      <select
        value={lang}
        onChange={(e) => setLang(e.target.value)}
      >
        <option value="es">ES</option>
        <option value="en">EN</option>
      </select>
    </label>
  );
}
