/** Build a nested JSON overlay from a compact path such as ietf-system:system/hostname. */
export function overlayFromPath(path: string, rawValue: string): Record<string, unknown> {
  const trimmedPath = path.trim();
  if (trimmedPath === "") {
    throw new Error("leaf path is required");
  }
  const colon = trimmedPath.indexOf(":");
  if (colon <= 0) {
    throw new Error("path must be module-qualified");
  }
  const module = trimmedPath.slice(0, colon).trim();
  if (module === "") {
    throw new Error("path must be module-qualified");
  }
  const rest = trimmedPath.slice(colon + 1);
  const segs = rest
    .split("/")
    .map((p) => p.trim())
    .filter((p) => p !== "")
    .map(segmentName);

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
  for (let i = segs.length - 1; i >= 0; i -= 1) {
    const key = segs[i];
    if (key === undefined) {
      continue;
    }
    cur = { [key]: cur };
  }
  return { [module]: cur };
}

function segmentName(seg: string): string {
  const bracket = seg.indexOf("[");
  if (bracket <= 0) {
    return seg;
  }
  return seg.slice(0, bracket);
}
