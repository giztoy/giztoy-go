import { Badge } from "../../../packages/components/badge";

export function StatusBadge({ status }: { status: string }): JSX.Element {
  if (status === "active") {
    return <Badge variant="success">Active</Badge>;
  }
  if (status === "blocked") {
    return <Badge variant="destructive">Blocked</Badge>;
  }
  return <Badge variant="outline">{status}</Badge>;
}
