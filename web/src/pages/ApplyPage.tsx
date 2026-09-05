import { FormEvent, useEffect, useState } from "react";
import { APIError, applyChanges, getState, planChanges, resetState } from "../api/client";
import type { Plan, StateView } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_ADMIN } from "../auth/scopes";
import { RESET_PHRASE, canSubmitReset } from "../ui/reset";

export function ApplyPage() {
  const { hasScope } = useAuth();
  const allowed = hasScope(SCOPE_ADMIN);
  const [state, setState] = useState<StateView | null>(null);
  const [ops, setOps] = useState("[]");
  const [plan, setPlan] = useState<Plan | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [phrase, setPhrase] = useState("");
  const [reason, setReason] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const resetOK = canSubmitReset(phrase, confirmed, allowed);

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
          setError(err instanceof APIError ? err.message : "Could not load revision.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function parseOps(): Promise<unknown[]> {
    const parsed = JSON.parse(ops) as unknown;
    if (!Array.isArray(parsed)) {
      throw new Error("operations must be a JSON array");
    }
    return parsed;
  }

  async function onPlan(ev: FormEvent) {
    ev.preventDefault();
    if (!allowed || state === null) {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const operations = await parseOps();
      const result = await planChanges(state.runtimeRevision, operations);
      setPlan(result);
      setNotice("Plan complete. Nothing was applied.");
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Plan failed.");
    } finally {
      setBusy(false);
    }
  }

  async function onApply() {
    if (!allowed || state === null) {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const operations = await parseOps();
      const key = crypto.randomUUID();
      const result = await applyChanges(state.runtimeRevision, operations, key);
      setPlan(result);
      setState(await getState());
      setNotice("Apply completed.");
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Apply failed.");
    } finally {
      setBusy(false);
    }
  }

  async function onReset(ev: FormEvent) {
    ev.preventDefault();
    if (!resetOK) {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await resetState(reason.trim());
      setState(await getState());
      setNotice("Reset completed. Bootstrap YAML was reread; trees restored; notifications wiped. The file was not written.");
      setPhrase("");
      setConfirmed(false);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Reset failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="page">
      <h1>Plan / apply / reset</h1>
      <p className="muted">
        Live apply ops only. ui.enabled and listener addresses are reset-only. Requires netconf.admin.
      </p>
      {!allowed ? <p>Requires scope netconf.admin.</p> : null}
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      {notice !== "" ? <p role="status">{notice}</p> : null}
      <p>
        expectedRevision: <code>{state?.runtimeRevision || "…"}</code>
      </p>
      <form className="stack" onSubmit={(e) => void onPlan(e)}>
        <div className="field">
          <label htmlFor="apply-ops">Operations JSON</label>
          <textarea id="apply-ops" value={ops} onChange={(e) => setOps(e.target.value)} />
        </div>
        <div className="row">
          <button type="submit" disabled={!allowed || busy || state === null}>
            Plan
          </button>
          <button type="button" disabled={!allowed || busy || state === null} onClick={() => void onApply()}>
            Apply
          </button>
        </div>
      </form>
      {plan ? <pre className="raw">{JSON.stringify(plan, null, 2)}</pre> : null}
      <h2>Reset</h2>
      <p>
        Reset rereads bootstrap YAML, restores running/candidate/startup, and wipes notifications. It never writes the
        file. Type <code>{RESET_PHRASE}</code> to enable the control.
      </p>
      <form className="stack" onSubmit={(e) => void onReset(e)}>
        <div className="field">
          <label htmlFor="reset-phrase">Confirmation phrase</label>
          <input
            id="reset-phrase"
            value={phrase}
            autoComplete="off"
            spellCheck={false}
            onChange={(e) => setPhrase(e.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor="reset-reason">Reason (optional)</label>
          <input id="reset-reason" value={reason} onChange={(e) => setReason(e.target.value)} />
        </div>
        <label>
          <input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} /> Wipe
          notifications and reread bootstrap
        </label>
        <button type="submit" disabled={!resetOK || busy}>
          {busy ? "Resetting…" : "Reset LabNETCONF"}
        </button>
      </form>
    </main>
  );
}
