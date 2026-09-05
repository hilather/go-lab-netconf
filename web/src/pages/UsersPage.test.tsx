import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { UsersPage } from "./UsersPage";

describe("UsersPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("shows secret paths and never password bytes", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/users")) {
          return json(200, {
            items: [
              {
                name: "alice",
                profile: "router-a",
                access: "read-write",
                passwordFile: "/run/secrets/netconf-alice",
              },
            ],
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "x" });
      }),
    );
    renderApp(<UsersPage />, { route: "/users" });
    expect(await screen.findByText("/run/secrets/netconf-alice")).toBeInTheDocument();
    expect(screen.queryByText(/supersecret/i)).toBeNull();
    expect(screen.queryByText(/BEGIN /)).toBeNull();
  });
});
