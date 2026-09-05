import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, seedCSRF, sessionView } from "../test/render";
import { ProfilesPage } from "./ProfilesPage";

describe("ProfilesPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("writes a candidate overlay from the leaf helper", async () => {
    const user = userEvent.setup();
    seedCSRF();
    const posts: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = (init?.method ?? "GET").toUpperCase();
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/profiles") && method === "GET") {
          return json(200, { items: [{ name: "router-a" }] });
        }
        if (url.endsWith("/v1/profiles/router-a") && method === "GET") {
          return json(200, { name: "router-a", instance: { "ietf-system": { system: { hostname: "lab-rtr-a" } } } });
        }
        if (url.endsWith("/v1/datastores/router-a/candidate:set") && method === "POST") {
          posts.push(String(init?.body ?? ""));
          return json(200, { ok: true });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "x" });
      }),
    );
    renderApp(<ProfilesPage />, { route: "/profiles" });
    await screen.findByText("Instance tree");
    await user.type(screen.getByLabelText(/Value/i), "via-ui");
    await user.click(screen.getByRole("button", { name: /Write candidate/i }));
    await waitFor(() => {
      expect(posts).toHaveLength(1);
    });
    expect(posts[0]).toContain("via-ui");
  });
});
