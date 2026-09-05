export type Problem = {
  type: string;
  title: string;
  status: number;
  detail: string;
  code: string;
};

export type SessionCreated = {
  csrf: string;
  expiresAt: string;
};

export type SessionView = {
  id: string;
  role: string;
  scopes: string[];
  csrf?: string;
  expiresAt?: string;
};

export type Listener = {
  name: string;
  address: string;
};

export type Status = {
  ready: boolean;
  revision?: string;
  listeners: Listener[];
};

export type Health = {
  status: string;
};

export type StateView = {
  bootstrapRevision: string;
  runtimeRevision: string;
  generation: number;
  drifted: boolean;
  loadedAt?: string;
  canonical?: unknown;
};

export type ProfileSummary = {
  name: string;
};

export type ProfileView = {
  name: string;
  modules?: unknown;
  schema?: unknown;
  instance?: unknown;
};

export type UserView = {
  name: string;
  profile: string;
  access: string;
  passwordFile?: string;
  authorizedKeysFile?: string;
};

export type NetconfSession = {
  id: string;
  user: string;
  profile: string;
};

export type NotificationChange = {
  path?: string;
  op?: string;
  value?: unknown;
};

export type Notification = {
  id: string;
  profile: string;
  changes: NotificationChange[];
};

export type ApplyOp = {
  op: string;
  [key: string]: unknown;
};

export type Plan = {
  previousRevision?: string;
  candidateRevision?: string;
  drifted?: boolean;
  diff?: unknown[];
  operations?: ApplyOp[];
  applied?: boolean;
  generation?: number;
  runtimeRevision?: string;
};

export type AuditEvent = {
  id: string;
  time?: string;
  actorId?: string;
  capability?: string;
  result?: string;
  errorCode?: string;
};
