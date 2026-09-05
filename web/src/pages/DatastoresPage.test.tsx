import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CSRF_HEADER } from "../api/client";
import { json, renderApp, resetClientState, seedCSRF, sessionView } from "../test/render";
import { DatastoresPage } from "./DatastoresPage";

describe("DatastoresPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("commits candidate with CSRF", async () => {
    const user = userEvent.setup();
    seedCSRF();
    const posts: Array<{ url: string; init: RequestInit | undefined }> = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = (init?.method ?? "GET").toUpperCase();
        if (url.endsWith("/v1/session") && method === "GET") {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/profiles") && method === "GET") {
          return json(200, { items: [{ name: "router-a" }] });
        }
        if (url.includes("/v1/datastores/router-a/") && method === "GET") {
          return json(200, { "ietf-system": { system: { hostname: "lab-rtr-a" } } });
        }
        if (method === "POST") {
          posts.push({ url, init });
          return json(200, { ok: true });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "x" });
      }),
    );
    renderApp(<DatastoresPage />, { route: "/datastores" });
    const commit = await screen.findByRole("button", { name: "Commit" });
    await user.click(commit);
    await waitFor(() => {
      expect(posts.some((p) => p.url.endsWith("/v1/datastores/router-a:commit"))).toBe(true);
    });
    const commitCall = posts.find((p) => p.url.endsWith("/v1/datastores/router-a:commit"));
    expect(new Headers(commitCall?.init?.headers).get(CSRF_HEADER)).toBe("csrf-test");
    expect(await screen.findByText("Committed candidate to running.")).toBeInTheDocument();
  });
});
