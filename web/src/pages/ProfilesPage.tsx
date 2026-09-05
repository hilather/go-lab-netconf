import { FormEvent, useEffect, useState } from "react";
import { APIError, getProfile, listProfiles, setDatastore } from "../api/client";
import type { ProfileSummary, ProfileView } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_WRITE } from "../auth/scopes";
import { overlayFromPath } from "../ui/overlay";

export function ProfilesPage() {
  const { hasScope } = useAuth();
  const canWrite = hasScope(SCOPE_WRITE);
  const [names, setNames] = useState<ProfileSummary[] | null>(null);
  const [selected, setSelected] = useState("");
  const [view, setView] = useState<ProfileView | null>(null);
  const [path, setPath] = useState("ietf-system:system/hostname");
  const [leaf, setLeaf] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listProfiles();
        if (cancelled) {
          return;
        }
        const items = list.items ?? [];
        setNames(items);
        if (items[0]) {
          setSelected(items[0].name);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load profiles.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (selected === "") {
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const p = await getProfile(selected);
        if (!cancelled) {
          setView(p);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load profile.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selected]);

  async function onEdit(ev: FormEvent) {
    ev.preventDefault();
    if (!canWrite || selected === "") {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const overlay = overlayFromPath(path, leaf);
      await setDatastore(selected, "candidate", overlay);
      setNotice("Wrote candidate overlay. Commit from Datastores to publish running.");
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Leaf edit failed.");
    } finally {
      setBusy(false);
    }
  }

  if (names === null && error === "") {
    return (
      <main className="page">
        <p role="status">Loading profiles…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Profiles</h1>
      <p className="muted">Device tree plus a compact-path leaf helper. Writes candidate via REST datastore:set.</p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      {notice !== "" ? <p role="status">{notice}</p> : null}
      <div className="field">
        <label htmlFor="profile-select">Profile</label>
        <select id="profile-select" value={selected} onChange={(e) => setSelected(e.target.value)}>
          {(names ?? []).map((p) => (
            <option key={p.name} value={p.name}>
              {p.name}
            </option>
          ))}
        </select>
      </div>
      {view ? (
        <>
          <h2>Instance tree</h2>
          <pre className="raw">{JSON.stringify(view.instance ?? {}, null, 2)}</pre>
          <h2>Leaf edit helper</h2>
          <form className="stack" onSubmit={(e) => void onEdit(e)}>
            <div className="field">
              <label htmlFor="leaf-path">Compact path</label>
              <input id="leaf-path" value={path} onChange={(e) => setPath(e.target.value)} />
            </div>
            <div className="field">
              <label htmlFor="leaf-value">Value (JSON or string)</label>
              <input id="leaf-value" value={leaf} onChange={(e) => setLeaf(e.target.value)} />
            </div>
            <button type="submit" disabled={!canWrite || busy || leaf.trim() === ""}>
              {busy ? "Writing…" : "Write candidate"}
            </button>
          </form>
        </>
      ) : null}
    </main>
  );
}
