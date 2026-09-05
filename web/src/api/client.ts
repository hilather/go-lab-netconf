import { assertNoTokenStorage } from "./storage";
import type {
  AuditEvent,
  Health,
  Notification,
  Plan,
  Problem,
  ProfileSummary,
  ProfileView,
  SessionCreated,
  SessionView,
  StateView,
  Status,
  UserView,
  NetconfSession,
} from "./types";

export const CSRF_HEADER = "X-LabNETCONF-CSRF";

export class APIError extends Error {
  readonly problem: Problem;

  constructor(problem: Problem) {
    super(problem.detail || problem.title || "request failed");
    this.name = "APIError";
    this.problem = problem;
  }
}

let memoryCSRF = "";

export function setMemoryCSRF(value: string): void {
  memoryCSRF = value;
}

export function getMemoryCSRF(): string {
  return memoryCSRF;
}

export function clearMemoryCSRF(): void {
  memoryCSRF = "";
}

function problemFrom(status: number, statusText: string, body: unknown): Problem {
  const fallback: Problem = {
    type: "urn:labnetconf:error:internal-error",
    title: statusText || "error",
    status,
    detail: statusText || "request failed",
    code: status === 401 ? "unauthorized" : status === 403 ? "forbidden" : "internal_error",
  };
  if (!body || typeof body !== "object") {
    return fallback;
  }
  const rec = body as Record<string, unknown>;
  return {
    type: typeof rec.type === "string" ? rec.type : fallback.type,
    title: typeof rec.title === "string" ? rec.title : fallback.title,
    status: typeof rec.status === "number" ? rec.status : fallback.status,
    detail: typeof rec.detail === "string" ? rec.detail : fallback.detail,
    code: typeof rec.code === "string" ? rec.code : fallback.code,
  };
}

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  assertNoTokenStorage();
  const headers = new Headers(init.headers);
  const method = (init.method ?? "GET").toUpperCase();
  if (method !== "GET" && method !== "HEAD" && !headers.has(CSRF_HEADER)) {
    const csrf = getMemoryCSRF();
    if (csrf !== "") {
      headers.set(CSRF_HEADER, csrf);
    }
  }
  if (!headers.has("Accept")) {
    headers.set("Accept", "application/json");
  }
  return fetch(path, {
    ...init,
    credentials: "same-origin",
    headers,
  });
}

async function readJSON<T>(resp: Response): Promise<T> {
  const text = await resp.text();
  let parsed: unknown;
  if (text !== "") {
    try {
      parsed = JSON.parse(text) as unknown;
    } catch {
      parsed = undefined;
    }
  }
  if (!resp.ok) {
    throw new APIError(problemFrom(resp.status, resp.statusText, parsed));
  }
  return parsed as T;
}

export async function createSession(authorization: string): Promise<SessionCreated> {
  const resp = await apiFetch("/v1/session", {
    method: "POST",
    headers: { Authorization: authorization },
  });
  const created = await readJSON<SessionCreated>(resp);
  setMemoryCSRF(created.csrf);
  assertNoTokenStorage();
  return created;
}

export function bearerAuthorization(token: string): string {
  return `Bearer ${token}`;
}

export async function getSession(): Promise<SessionView> {
  const view = await readJSON<SessionView>(await apiFetch("/v1/session"));
  if (typeof view.csrf === "string" && view.csrf !== "") {
    setMemoryCSRF(view.csrf);
  }
  return view;
}

export async function deleteSession(): Promise<void> {
  const resp = await apiFetch("/v1/session", { method: "DELETE" });
  if (resp.status === 401 || resp.status === 204) {
    clearMemoryCSRF();
    return;
  }
  await readJSON<unknown>(resp);
  clearMemoryCSRF();
}

export async function getHealth(kind: "live" | "ready"): Promise<Health> {
  return readJSON<Health>(await apiFetch(`/v1/health/${kind}`));
}

export async function getStatus(): Promise<Status> {
  return readJSON<Status>(await apiFetch("/v1/status"));
}

export async function getState(): Promise<StateView> {
  return readJSON<StateView>(await apiFetch("/v1/state"));
}

export async function listProfiles(): Promise<{ items: ProfileSummary[] }> {
  return readJSON<{ items: ProfileSummary[] }>(await apiFetch("/v1/profiles"));
}

export async function getProfile(name: string): Promise<ProfileView> {
  return readJSON<ProfileView>(await apiFetch(`/v1/profiles/${encodeURIComponent(name)}`));
}

export async function listUsers(): Promise<{ items: UserView[] }> {
  return readJSON<{ items: UserView[] }>(await apiFetch("/v1/users"));
}

export async function getDatastore(profile: string, store: string): Promise<unknown> {
  return readJSON<unknown>(
    await apiFetch(`/v1/datastores/${encodeURIComponent(profile)}/${encodeURIComponent(store)}`),
  );
}

export async function setDatastore(profile: string, store: string, overlay: unknown): Promise<unknown> {
  return readJSON<unknown>(
    await apiFetch(`/v1/datastores/${encodeURIComponent(profile)}/${encodeURIComponent(store)}:set`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(overlay),
    }),
  );
}

export async function commitDatastore(profile: string): Promise<unknown> {
  return readJSON<unknown>(
    await apiFetch(`/v1/datastores/${encodeURIComponent(profile)}:commit`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    }),
  );
}

export async function discardDatastore(profile: string): Promise<unknown> {
  return readJSON<unknown>(
    await apiFetch(`/v1/datastores/${encodeURIComponent(profile)}:discard`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    }),
  );
}

export async function listSessions(): Promise<{ items: NetconfSession[] }> {
  return readJSON<{ items: NetconfSession[] }>(await apiFetch("/v1/sessions"));
}

export async function killSession(id: string): Promise<unknown> {
  return readJSON<unknown>(
    await apiFetch(`/v1/sessions/${encodeURIComponent(id)}:kill`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    }),
  );
}

export async function listNotifications(profile = ""): Promise<{ items: Notification[] }> {
  const params = new URLSearchParams();
  if (profile !== "") {
    params.set("profile", profile);
  }
  const q = params.toString();
  return readJSON<{ items: Notification[] }>(await apiFetch(`/v1/notifications${q ? `?${q}` : ""}`));
}

export async function waitNotification(profile: string, timeout: string): Promise<Notification> {
  return readJSON<Notification>(
    await apiFetch("/v1/notifications:wait", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ profile, timeout }),
    }),
  );
}

export async function planChanges(expectedRevision: string, operations: unknown[]): Promise<Plan> {
  return readJSON<Plan>(
    await apiFetch("/v1/changes:plan", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ expectedRevision, operations }),
    }),
  );
}

export async function applyChanges(
  expectedRevision: string,
  operations: unknown[],
  idempotencyKey: string,
): Promise<Plan> {
  return readJSON<Plan>(
    await apiFetch("/v1/changes:apply", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify({ expectedRevision, operations }),
    }),
  );
}

export async function resetState(reason: string): Promise<unknown> {
  return readJSON<unknown>(
    await apiFetch("/v1/state:reset", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ reason }),
    }),
  );
}

export async function listAudit(): Promise<{ events: AuditEvent[] }> {
  return readJSON<{ events: AuditEvent[] }>(await apiFetch("/v1/audit"));
}
