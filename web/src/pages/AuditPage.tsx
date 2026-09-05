import { useEffect, useState } from "react";
import { APIError, listAudit } from "../api/client";
import type { AuditEvent } from "../api/types";

export function AuditPage() {
  const [events, setEvents] = useState<AuditEvent[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listAudit();
        if (!cancelled) {
          setEvents(list.events ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load audit.");
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
  if (events === null) {
    return (
      <main className="page">
        <p role="status">Loading audit…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Audit</h1>
      <p className="muted">In-process mutation ring. Newest first. Requires netconf.audit.read.</p>
      {events.length === 0 ? (
        <p className="muted">No events.</p>
      ) : (
        <table className="data">
          <thead>
            <tr>
              <th>Time</th>
              <th>Actor</th>
              <th>Capability</th>
              <th>Result</th>
            </tr>
          </thead>
          <tbody>
            {events.map((e) => (
              <tr key={e.id}>
                <td>
                  <code>{e.time || e.id}</code>
                </td>
                <td>{e.actorId || "—"}</td>
                <td>{e.capability || "—"}</td>
                <td>
                  {e.result || "—"}
                  {e.errorCode ? ` (${e.errorCode})` : ""}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  );
}
