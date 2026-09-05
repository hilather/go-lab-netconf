import { useEffect, useState } from "react";
import { APIError, getHealth, getState, getStatus, listSessions } from "../api/client";
import type { Health, NetconfSession, StateView, Status } from "../api/types";

export function OverviewPage() {
  const [live, setLive] = useState<Health | null>(null);
  const [ready, setReady] = useState<Health | null>(null);
  const [status, setStatus] = useState<Status | null>(null);
  const [state, setState] = useState<StateView | null>(null);
  const [sessions, setSessions] = useState<NetconfSession[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const [lv, rd, st, sv, sess] = await Promise.all([
          getHealth("live"),
          getHealth("ready"),
          getStatus(),
          getState(),
          listSessions(),
        ]);
        if (!cancelled) {
          setLive(lv);
          setReady(rd);
          setStatus(st);
          setState(sv);
          setSessions(sess.items ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load overview.");
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
  if (live === null || ready === null || status === null || state === null || sessions === null) {
    return (
      <main className="page">
        <p role="status">Loading overview…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Overview</h1>
      <p className="muted">Listeners, sessions, revision, and health. REST only — this process never dials a peer.</p>
      <dl>
        <div>
          <dt>Live</dt>
          <dd>
            <strong>{live.status}</strong>
          </dd>
        </div>
        <div>
          <dt>Ready</dt>
          <dd>
            <strong>{ready.status}</strong>
          </dd>
        </div>
        <div>
          <dt>Revision</dt>
          <dd>
            <code>{status.revision || state.runtimeRevision}</code>
          </dd>
        </div>
        <div>
          <dt>Sessions</dt>
          <dd>{sessions.length}</dd>
        </div>
      </dl>
      <h2>Listeners</h2>
      {(status.listeners ?? []).length === 0 ? (
        <p className="muted">None advertised.</p>
      ) : (
        <ul>
          {(status.listeners ?? []).map((l) => (
            <li key={l.name}>
              {l.name}: <code>{l.address}</code>
            </li>
          ))}
        </ul>
      )}
      <h2>Sessions</h2>
      {sessions.length === 0 ? (
        <p className="muted">No NETCONF or RESTCONF sessions.</p>
      ) : (
        <table className="data">
          <thead>
            <tr>
              <th>ID</th>
              <th>User</th>
              <th>Profile</th>
            </tr>
          </thead>
          <tbody>
            {sessions.map((s) => (
              <tr key={s.id}>
                <td>
                  <code>{s.id}</code>
                </td>
                <td>{s.user}</td>
                <td>{s.profile}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  );
}
