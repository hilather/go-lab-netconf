import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, seedCSRF, sessionView } from "../test/render";
import { NotificationsPage } from "./NotificationsPage";

describe("NotificationsPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("waits for a config-change record", async () => {
    const user = userEvent.setup();
    seedCSRF();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = (init?.method ?? "GET").toUpperCase();
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/notifications") && method === "GET") {
          return json(200, { items: [] });
        }
        if (url.endsWith("/v1/notifications:wait") && method === "POST") {
          return json(200, { id: "01WAIT", profile: "router-a", changes: [{ path: "hostname" }] });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "x" });
      }),
    );
    renderApp(<NotificationsPage />, { route: "/notifications" });
    await user.click(await screen.findByRole("button", { name: "Wait" }));
    await waitFor(() => {
      expect(screen.getByText("01WAIT")).toBeInTheDocument();
    });
  });
});
