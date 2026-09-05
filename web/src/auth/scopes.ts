export const SCOPE_READ = "netconf.read";
export const SCOPE_WRITE = "netconf.write";
export const SCOPE_ADMIN = "netconf.admin";
export const SCOPE_AUDIT = "netconf.audit.read";

export function hasScope(scopes: readonly string[], need: string): boolean {
  return scopes.includes(SCOPE_ADMIN) || scopes.includes(need);
}
