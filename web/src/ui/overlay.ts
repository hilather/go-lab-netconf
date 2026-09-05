/** Build a nested JSON overlay from a compact path such as ietf-system:system/hostname. */
export function overlayFromPath(path: string, rawValue: string): Record<string, unknown> {
  const parts = path
    .split("/")
    .map((p) => p.trim())
    .filter((p) => p !== "");
  if (parts.length === 0) {
    throw new Error("leaf path is required");
  }
  let value: unknown = rawValue;
  const trimmed = rawValue.trim();
  if (trimmed !== "") {
    try {
      value = JSON.parse(trimmed) as unknown;
    } catch {
      value = rawValue;
    }
  }
  let cur: unknown = value;
  for (let i = parts.length - 1; i >= 0; i -= 1) {
    const key = parts[i];
    if (key === undefined) {
      continue;
    }
    cur = { [key]: cur };
  }
  return cur as Record<string, unknown>;
}
