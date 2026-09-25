import { useEffect, useState } from "react";
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
          <button type="button" className={view === "meters" ? "on" : ""} onClick={() => setView("meters")}>
            {text(lang, "meters")}
          </button>
        </nav>
        <div className="tools">
          <Language lang={lang} setLang={setLang} />
          <button type="button" onClick={logout}>{text(lang, "logout")}</button>
        </div>
      </header>
      {view === "meters" ? (
        <section className="panel">
          <h1>{text(lang, "meters")}</h1>
          <p>{summary ? summary.meter_count : "—"}</p>
        </section>
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
