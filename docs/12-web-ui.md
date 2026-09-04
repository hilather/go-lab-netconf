# 12 — Operator UI

Required for 1.0 GA. Pages: overview (listeners, sessions, revision),
profiles (tree + leaf edit helper), users (secrets never shown),
datastores (running/candidate/startup + commit/discard),
notification inbox, session list, plan/apply/reset, audit.

REST only. No localStorage tokens. No "call-home" or "send RPC to
remote" control. `ui.enabled: false` hides the SPA.
