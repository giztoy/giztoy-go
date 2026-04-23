import type { Depot } from "../../../packages/adminservice";

import { hasRelease } from "./format";

export function canReleaseDepot(depot: Depot): boolean {
  return hasRelease(depot.beta) && hasRelease(depot.testing);
}

export function canRollbackDepot(depot: Depot): boolean {
  return hasRelease(depot.rollback);
}

export function depotActionHint(depot: Depot): string {
  if (!canReleaseDepot(depot)) {
    const missing: string[] = [];
    if (!hasRelease(depot.beta)) {
      missing.push("beta");
    }
    if (!hasRelease(depot.testing)) {
      missing.push("testing");
    }
    return `Release requires ${missing.join(" + ")}.`;
  }
  if (!canRollbackDepot(depot)) {
    return "Rollback requires a rollback snapshot.";
  }
  return "Ready for release and rollback.";
}
