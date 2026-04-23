import { ChevronRight, RefreshCw, Upload } from "lucide-react";
import { Link } from "react-router-dom";

import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../../packages/components/card";
import { Skeleton } from "../../../../packages/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../../packages/components/table";

import { EmptyState } from "../../components/empty-state";
import { ErrorBanner } from "../../components/banners";
import { PageBreadcrumb } from "../../components/page-breadcrumb";
import { useFirmwareList } from "../../hooks/useFirmwareList";
import { formatRelease } from "../../lib/format";
import { canReleaseDepot, canRollbackDepot, depotActionHint } from "../../lib/firmware-helpers";

export function FirmwareListPage(): JSX.Element {
  const { depots, error, loading, reload } = useFirmwareList();

  return (
    <div className="space-y-6">
      <PageBreadcrumb items={[{ href: "/overview", label: "Overview" }, { label: "Firmware" }]} />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Firmware</div>
          <h1 className="text-3xl font-semibold tracking-tight">Depots</h1>
          <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">
            Browse firmware depots. Open a depot to manage metadata, release operations, and channel uploads.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button asChild className="h-8 min-w-fit shrink-0 whitespace-nowrap px-3 text-sm" variant="default">
            <Link to="/firmware/new">
              <span className="inline-flex items-center gap-2 whitespace-nowrap">
                <Upload className="size-4" />
                Add firmware
              </span>
            </Link>
          </Button>
          <Button className="h-8 min-w-fit shrink-0 whitespace-nowrap px-3 text-sm" onClick={() => void reload()} variant="outline">
            <span className="inline-flex items-center gap-2 whitespace-nowrap">
              <RefreshCw className="size-4" />
              Refresh
            </span>
          </Button>
        </div>
      </div>

      {error !== "" ? <ErrorBanner message={error} /> : null}

      <Card>
        <CardHeader>
          <CardTitle>Firmware depots</CardTitle>
          <CardDescription>Each row links to the depot workspace for three release channels plus the separate rollback action.</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 6 }).map((_, index) => (
                <Skeleton className="h-12 w-full" key={index} />
              ))}
            </div>
          ) : depots.length === 0 ? (
            <EmptyState description="Firmware depot snapshots will appear here when release data is available." title="No firmware depots" />
          ) : (
            <div className="rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Depot</TableHead>
                    <TableHead>Stable</TableHead>
                    <TableHead>Beta</TableHead>
                    <TableHead>Testing</TableHead>
                    <TableHead>Rollback Snapshot</TableHead>
                    <TableHead className="text-right">Files</TableHead>
                    <TableHead className="text-right">Readiness</TableHead>
                    <TableHead className="text-right">Open</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {depots.map((depot) => {
                    const depotPath = encodeURIComponent(depot.name);
                    return (
                      <TableRow className="cursor-pointer hover:bg-muted/40" key={depot.name}>
                        <TableCell>
                          <Link className="font-medium hover:underline" to={`/firmware/${depotPath}`}>
                            {depot.name}
                          </Link>
                        </TableCell>
                        <TableCell>{formatRelease(depot.stable)}</TableCell>
                        <TableCell>{formatRelease(depot.beta)}</TableCell>
                        <TableCell>{formatRelease(depot.testing)}</TableCell>
                        <TableCell>{formatRelease(depot.rollback)}</TableCell>
                        <TableCell className="text-right">{depot.info?.files?.length ?? 0}</TableCell>
                        <TableCell className="max-w-[14rem] text-right text-xs text-muted-foreground">
                          {canReleaseDepot(depot) && canRollbackDepot(depot) ? "Ready" : depotActionHint(depot)}
                        </TableCell>
                        <TableCell className="text-right text-muted-foreground">
                          <Link to={`/firmware/${depotPath}`}>
                            <ChevronRight className="ml-auto size-4" />
                          </Link>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
