# SSH-001 — SSH NETCONF server

Status: not-started
Depends: FRAME-001, DS-001
Owns: internal/netconfssh, internal/ncserver

Listen TCP, SSH handshake, subsystem `netconf`, session hello, RPC
dispatch, user → model. Interop with ncclient. Isolation. Host key
file-ref. Management-off still answers.
