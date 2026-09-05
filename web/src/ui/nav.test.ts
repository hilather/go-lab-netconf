import { describe, expect, it } from "vitest";
import { navItems } from "./nav";
import { overlayFromPath } from "./overlay";
import { canSubmitReset } from "./reset";

describe("operator nav", () => {
  it("matches docs/12 pages and omits gated items", () => {
    const labels = navItems(false, false).map((i) => i.label);
    expect(labels).toEqual([
      "Overview",
      "State",
      "Profiles",
      "Users",
      "Datastores",
      "Notifications",
      "Sessions",
    ]);
    expect(navItems(true, true).map((i) => i.label)).toEqual([...labels, "Apply", "Audit"]);
    expect(labels.join(" ")).not.toMatch(/call-home|send rpc|remote/i);
  });

  it("gates reset on the exact phrase and confirmation", () => {
    expect(canSubmitReset("RESET", true, true)).toBe(true);
    expect(canSubmitReset("reset", true, true)).toBe(false);
    expect(canSubmitReset("RESET", false, true)).toBe(false);
    expect(canSubmitReset("RESET", true, false)).toBe(false);
  });

  it("builds a leaf overlay from a compact path", () => {
    expect(overlayFromPath("ietf-system:system/hostname", '"lab-rtr-a"')).toEqual({
      "ietf-system:system": { hostname: "lab-rtr-a" },
    });
    expect(overlayFromPath("ietf-system:system/hostname", "lab-rtr-a")).toEqual({
      "ietf-system:system": { hostname: "lab-rtr-a" },
    });
  });
});
