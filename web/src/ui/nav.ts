export type NavItem = { to: string; label: string };

export function navItems(canApply: boolean, canAudit: boolean): NavItem[] {
  const items: NavItem[] = [
    { to: "/", label: "Overview" },
    { to: "/state", label: "State" },
    { to: "/profiles", label: "Profiles" },
    { to: "/users", label: "Users" },
    { to: "/datastores", label: "Datastores" },
    { to: "/notifications", label: "Notifications" },
    { to: "/sessions", label: "Sessions" },
  ];
  if (canApply) {
    items.push({ to: "/apply", label: "Apply" });
  }
  if (canAudit) {
    items.push({ to: "/audit", label: "Audit" });
  }
  return items;
}
