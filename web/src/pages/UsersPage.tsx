import { useEffect, useState } from "react";
import { APIError, listUsers } from "../api/client";
import type { UserView } from "../api/types";

export function UsersPage() {
  const [users, setUsers] = useState<UserView[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listUsers();
        if (!cancelled) {
          setUsers(list.items ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load users.");
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
  if (users === null) {
    return (
      <main className="page">
        <p role="status">Loading users…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Users</h1>
      <p className="muted">Data-plane users. Password and key file contents are never shown.</p>
      <table className="data">
        <caption>Secret bytes stay on disk; only paths are listed.</caption>
        <thead>
          <tr>
            <th>Name</th>
            <th>Profile</th>
            <th>Access</th>
            <th>passwordFile</th>
            <th>authorizedKeysFile</th>
          </tr>
        </thead>
        <tbody>
          {users.map((u) => (
            <tr key={u.name}>
              <td>{u.name}</td>
              <td>{u.profile}</td>
              <td>{u.access}</td>
              <td>
                <code>{u.passwordFile || "—"}</code>
              </td>
              <td>
                <code>{u.authorizedKeysFile || "—"}</code>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </main>
  );
}
