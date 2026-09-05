import { FormEvent, useEffect, useState } from "react";
import { APIError, killSession, listSessions } from "../api/client";
import type { NetconfSession } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_ADMIN } from "../auth/scopes";

export function SessionsPage() {
  const { hasScope } = useAuth();
  const canKill = hasScope(SCOPE_ADMIN);
  const [items, setItems] = useState<NetconfSession[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");

  async function refresh() {
    const list = await listSessions();
    setItems(list.items ?? []);
  }

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listSessions();
        if (!cancelled) {
          setItems(list.items ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load sessions.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function onKill(ev: FormEvent, id: string) {
    ev.preventDefault();
    if (!canKill) {
      return;
    }
    setBusy(id);
    setError("");
    try {
      await killSession(id);
      await refresh();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Kill failed.");
    } finally {
      setBusy("");
    }
  }

  if (items === null && error === "") {
    return (
      <main className="page">
        <p role="status">Loading sessions…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Sessions</h1>
      <p className="muted">NETCONF SSH and RESTCONF HTTP sessions. Kill requires netconf.admin.</p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      {(items ?? []).length === 0 ? (
        <p className="muted">No sessions.</p>
      ) : (
        <table className="data">
          <thead>
            <tr>
              <th>ID</th>
              <th>User</th>
              <th>Profile</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {(items ?? []).map((s) => (
              <tr key={s.id}>
                <td>
                  <code>{s.id}</code>
                </td>
                <td>{s.user}</td>
                <td>{s.profile}</td>
                <td>
                  <button type="button" disabled={!canKill || busy === s.id} onClick={(e) => void onKill(e, s.id)}>
                    Kill
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  );
}
