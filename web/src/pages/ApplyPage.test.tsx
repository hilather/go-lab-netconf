import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { ApplyPage } from "./ApplyPage";

describe("ApplyPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("keeps reset disabled until RESET is typed and confirmed", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input).endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (String(input).endsWith("/v1/state")) {
          return json(200, {
            bootstrapRevision: "sha256:boot",
            runtimeRevision: "sha256:run",
            generation: 1,
            drifted: false,
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "x" });
      }),
    );
    renderApp(<ApplyPage />, { route: "/apply" });
    const submit = await screen.findByRole("button", { name: /Reset LabNETCONF/i });
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText(/Confirmation phrase/i), "RESET");
    expect(submit).toBeDisabled();
    await user.click(screen.getByLabelText(/Wipe notifications/i));
    expect(submit).toBeEnabled();
  });

  it("keeps reset disabled without netconf.admin", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input).endsWith("/v1/session")) {
          return json(200, sessionView(["netconf.read", "netconf.write"]));
        }
        if (String(input).endsWith("/v1/state")) {
          return json(200, {
            bootstrapRevision: "sha256:boot",
            runtimeRevision: "sha256:run",
            generation: 1,
            drifted: false,
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "x" });
      }),
    );
    renderApp(<ApplyPage />, { route: "/apply" });
    const submit = await screen.findByRole("button", { name: /Reset LabNETCONF/i });
    expect(submit).toBeDisabled();
  });
});
