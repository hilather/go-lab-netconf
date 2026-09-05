import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { json, resetClientState, sessionView } from "./test/render";

describe("App nav", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("hides Apply without netconf.admin", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView(["netconf.read"]));
        }
        if (url.endsWith("/v1/health/live") || url.endsWith("/v1/health/ready")) {
          return json(200, { status: "ok" });
        }
        if (url.endsWith("/v1/status")) {
          return json(200, { ready: true, revision: "sha256:x", listeners: [] });
        }
        if (url.endsWith("/v1/state")) {
          return json(200, { bootstrapRevision: "sha256:b", runtimeRevision: "sha256:r", generation: 1, drifted: false });
        }
        if (url.endsWith("/v1/sessions")) {
          return json(200, { items: [] });
        }
        return json(404, {
          status: 404,
          title: "not found",
          detail: "not found",
          code: "not_found",
          type: "urn:labnetconf:error:not-found",
        });
      }),
    );
    render(<App />);
    expect(await screen.findByRole("link", { name: "Overview" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Apply" })).toBeNull();
    expect(screen.queryByRole("link", { name: /call-home/i })).toBeNull();
  });
});
