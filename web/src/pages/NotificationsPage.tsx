import { FormEvent, useEffect, useState } from "react";
import { APIError, listNotifications, waitNotification } from "../api/client";
import type { Notification } from "../api/types";

export function NotificationsPage() {
  const [items, setItems] = useState<Notification[] | null>(null);
  const [profile, setProfile] = useState("");
  const [timeout, setTimeoutValue] = useState("5s");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listNotifications();
        if (!cancelled) {
          setItems(list.items ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load notifications.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function onWait(ev: FormEvent) {
    ev.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const n = await waitNotification(profile, timeout);
      setItems((cur) => [n, ...(cur ?? []).filter((x) => x.id !== n.id)]);
      setNotice(`Wait returned ${n.id}.`);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Wait failed.");
    } finally {
      setBusy(false);
    }
  }

  if (items === null && error === "") {
    return (
      <main className="page">
        <p role="status">Loading notifications…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Notifications</h1>
      <p className="muted">In-process RFC 5277 inbox. Wait blocks until a config-change, timeout, or wipe.</p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      {notice !== "" ? <p role="status">{notice}</p> : null}
      <form className="row" onSubmit={(e) => void onWait(e)}>
        <div className="field">
          <label htmlFor="wait-profile">Profile (optional)</label>
          <input id="wait-profile" value={profile} onChange={(e) => setProfile(e.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="wait-timeout">Timeout</label>
          <input id="wait-timeout" value={timeout} onChange={(e) => setTimeoutValue(e.target.value)} />
        </div>
        <button type="submit" disabled={busy}>
          {busy ? "Waiting…" : "Wait"}
        </button>
      </form>
      {(items ?? []).length === 0 ? (
        <p className="muted">Inbox is empty.</p>
      ) : (
        <table className="data">
          <thead>
            <tr>
              <th>ID</th>
              <th>Profile</th>
              <th>Changes</th>
            </tr>
          </thead>
          <tbody>
            {(items ?? []).map((n) => (
              <tr key={n.id}>
                <td>
                  <code>{n.id}</code>
                </td>
                <td>{n.profile}</td>
                <td>{(n.changes ?? []).length}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  );
}
