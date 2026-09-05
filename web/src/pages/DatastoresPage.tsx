import { FormEvent, useEffect, useState } from "react";
import { APIError, commitDatastore, discardDatastore, getDatastore, listProfiles, setDatastore } from "../api/client";
import type { ProfileSummary } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_WRITE } from "../auth/scopes";

const STORES = ["running", "candidate", "startup"] as const;

export function DatastoresPage() {
  const { hasScope } = useAuth();
  const canWrite = hasScope(SCOPE_WRITE);
  const [profiles, setProfiles] = useState<ProfileSummary[] | null>(null);
  const [profile, setProfile] = useState("");
  const [store, setStore] = useState<(typeof STORES)[number]>("running");
  const [tree, setTree] = useState<unknown>(null);
  const [overlay, setOverlay] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);

  async function loadTree(p: string, st: string) {
    const raw = await getDatastore(p, st);
    setTree(raw);
  }

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listProfiles();
        if (cancelled) {
          return;
        }
        const items = list.items ?? [];
        setProfiles(items);
        const first = items[0]?.name ?? "";
        setProfile(first);
        if (first !== "") {
          setTree(await getDatastore(first, "running"));
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load datastores.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function onStoreChange(next: (typeof STORES)[number]) {
    setStore(next);
    if (profile === "") {
      return;
    }
    setError("");
    try {
      await loadTree(profile, next);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Could not load store.");
    }
  }

  async function onProfileChange(next: string) {
    setProfile(next);
    if (next === "") {
      return;
    }
    setError("");
    try {
      await loadTree(next, store);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Could not load store.");
    }
  }

  async function onSet(ev: FormEvent) {
    ev.preventDefault();
    if (!canWrite || profile === "") {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const body = JSON.parse(overlay) as unknown;
      await setDatastore(profile, store, body);
      await loadTree(profile, store);
      setNotice(`Wrote ${store}.`);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Set failed (overlay must be JSON).");
    } finally {
      setBusy(false);
    }
  }

  async function onCommit() {
    if (!canWrite || profile === "") {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await commitDatastore(profile);
      await loadTree(profile, store);
      setNotice("Committed candidate to running.");
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Commit failed.");
    } finally {
      setBusy(false);
    }
  }

  async function onDiscard() {
    if (!canWrite || profile === "") {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await discardDatastore(profile);
      await loadTree(profile, store);
      setNotice("Discarded candidate back to running.");
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Discard failed.");
    } finally {
      setBusy(false);
    }
  }

  if (profiles === null && error === "") {
    return (
      <main className="page">
        <p role="status">Loading datastores…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Datastores</h1>
      <p className="muted">running / candidate / startup. Commit publishes candidate; discard restores it.</p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      {notice !== "" ? <p role="status">{notice}</p> : null}
      <div className="row">
        <div className="field">
          <label htmlFor="ds-profile">Profile</label>
          <select id="ds-profile" value={profile} onChange={(e) => void onProfileChange(e.target.value)}>
            {(profiles ?? []).map((p) => (
              <option key={p.name} value={p.name}>
                {p.name}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor="ds-store">Store</label>
          <select id="ds-store" value={store} onChange={(e) => void onStoreChange(e.target.value as (typeof STORES)[number])}>
            {STORES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </div>
        <button type="button" disabled={!canWrite || busy} onClick={() => void onCommit()}>
          Commit
        </button>
        <button type="button" disabled={!canWrite || busy} onClick={() => void onDiscard()}>
          Discard
        </button>
      </div>
      <pre className="raw">{JSON.stringify(tree ?? {}, null, 2)}</pre>
      <form className="stack" onSubmit={(e) => void onSet(e)}>
        <div className="field">
          <label htmlFor="ds-overlay">JSON overlay for :set</label>
          <textarea id="ds-overlay" value={overlay} onChange={(e) => setOverlay(e.target.value)} />
        </div>
        <button type="submit" disabled={!canWrite || busy || overlay.trim() === ""}>
          {busy ? "Writing…" : "Set overlay"}
        </button>
      </form>
    </main>
  );
}
