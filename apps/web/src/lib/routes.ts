export const primaryRoutes = [
  { href: "/", label: "Home" },
  { href: "/send", label: "Send" },
  { href: "/receive/demo-code", label: "Receive" },
  { href: "/ops", label: "Ops" },
] as const;

export function buildReceivePath(code: string) {
  return `/receive/${encodeURIComponent(code)}`;
}
