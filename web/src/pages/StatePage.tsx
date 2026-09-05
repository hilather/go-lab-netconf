import { useEffect, useState } from "react";
import { APIError, getState } from "../api/client";
import type { StateView } from "../api/types";

export function StatePage() {
  const [state, setState] = useState<StateView | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const sv = await getState();
        if (!cancelled) {
          setState(sv);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load state.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (error !== "") {
    return (
      <main className="page">
        <p className="banner-error" role="alert">
          {error}
        </p>
      </main>
    );
  }
  if (state === null) {
    return (
      <main className="page">
        <p role="status">Loading state…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>State</h1>
      <p className="muted">Redacted spec. Secret bytes are omitted; file paths remain.</p>
      <dl>
        <div>
          <dt>Bootstrap</dt>
          <dd>
            <code>{state.bootstrapRevision}</code>
          </dd>
        </div>
        <div>
          <dt>Runtime</dt>
          <dd>
            <code>{state.runtimeRevision}</code>
          </dd>
        </div>
        <div>
          <dt>Generation</dt>
          <dd>{state.generation}</dd>
        </div>
        <div>
          <dt>Drifted</dt>
          <dd>{state.drifted ? "yes" : "no"}</dd>
        </div>
      </dl>
      <pre className="raw">{JSON.stringify(state.canonical ?? {}, null, 2)}</pre>
    </main>
  );
}
