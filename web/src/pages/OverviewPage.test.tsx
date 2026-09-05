import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { OverviewPage } from "./OverviewPage";

describe("OverviewPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("renders health, listeners, revision, and sessions", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/health/live") || url.endsWith("/v1/health/ready")) {
          return json(200, { status: "ok" });
        }
        if (url.endsWith("/v1/status")) {
          return json(200, {
            ready: true,
            revision: "sha256:rev",
            listeners: [{ name: "netconf", address: ":830" }],
          });
        }
        if (url.endsWith("/v1/state")) {
          return json(200, {
            bootstrapRevision: "sha256:boot",
            runtimeRevision: "sha256:rev",
            generation: 1,
            drifted: false,
          });
        }
        if (url.endsWith("/v1/sessions")) {
          return json(200, { items: [{ id: "s1", user: "alice", profile: "router-a" }] });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "x" });
      }),
    );
    renderApp(<OverviewPage />);
    expect(await screen.findByText("Overview")).toBeInTheDocument();
    expect(screen.getByText(":830")).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(screen.getAllByText("ok").length).toBeGreaterThan(0);
  });
});
