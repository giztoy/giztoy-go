import { ChevronRight, RefreshCw, Search } from "lucide-react";
import { Link } from "react-router-dom";

import { Badge } from "../../../../packages/components/badge";
import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardTitle } from "../../../../packages/components/card";
import { Input } from "../../../../packages/components/input";
import { Skeleton } from "../../../../packages/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../../packages/components/table";

import { EmptyState } from "../../../../packages/components/empty-state";
import { PageBreadcrumb } from "../../../../packages/components/page-breadcrumb";
import { StatusBadge } from "../../../../packages/components/status-badge";
import { useDevicesPage } from "../../hooks/useDevicesPage";
import { deviceTitle, formatDate } from "../../lib/format";

export function DevicesListPage(): JSX.Element {
  const {
    dashboard,
    deviceList,
    devicePageNumber,
    filter,
    filteredGears,
    nextPage,
    prevPage,
    refreshDashboard,
    setFilter,
  } = useDevicesPage();

  return (
    <div className="space-y-6">
      <PageBreadcrumb items={[{ href: "/overview", label: "Overview" }, { label: "Devices" }]} />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Inventory</div>
          <h1 className="text-3xl font-semibold tracking-tight">Devices</h1>
          <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">
            Browse paged inventory, filter the current page, and open a device into its own detail route.
          </p>
        </div>
        <Button className="h-8 min-w-fit shrink-0 whitespace-nowrap px-3 text-sm" onClick={() => void refreshDashboard()} variant="outline">
          <span className="inline-flex items-center gap-2 whitespace-nowrap">
            <RefreshCw className="size-4" />
            Refresh
          </span>
        </Button>
      </div>

      {dashboard.error !== "" ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">{dashboard.error}</div>
      ) : null}

      <Card>
        <CardContent className="p-6">
          <div className="rounded-md border">
            <div className="flex flex-col gap-3 border-b px-4 py-4 lg:flex-row lg:items-start lg:justify-between">
              <div className="space-y-1">
                <CardTitle>Device Inventory</CardTitle>
                <CardDescription>Browse paged device results and open a row to inspect details.</CardDescription>
              </div>
              <div className="flex flex-wrap gap-2">
                <Badge variant="outline">Page {devicePageNumber}</Badge>
                <Badge variant="secondary">{dashboard.gears.length} loaded</Badge>
                {deviceList.hasNext ? <Badge variant="outline">More Available</Badge> : null}
              </div>
            </div>

            <div className="flex flex-col gap-3 border-b px-4 py-4 lg:flex-row lg:items-center lg:justify-between">
              <div className="relative lg:max-w-sm lg:flex-1">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="h-8 pl-9"
                  onChange={(event) => setFilter(event.target.value)}
                  placeholder="Filter current page by key, role, or status"
                  value={filter}
                />
              </div>
              <div className="flex gap-2">
                <Button
                  className="h-8 min-w-fit shrink-0 whitespace-nowrap px-3 text-sm disabled:border-border disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100 disabled:shadow-none"
                  disabled={dashboard.loading || deviceList.history.length === 0}
                  onClick={prevPage}
                  type="button"
                  variant="outline"
                >
                  Previous
                </Button>
                <Button
                  className="h-8 min-w-fit shrink-0 whitespace-nowrap px-3 text-sm disabled:border-border disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100 disabled:shadow-none"
                  disabled={dashboard.loading || !deviceList.hasNext || deviceList.nextCursor === null}
                  onClick={nextPage}
                  type="button"
                  variant="outline"
                >
                  Next
                </Button>
              </div>
            </div>

            {dashboard.loading ? (
              <div className="space-y-3 p-4">
                {Array.from({ length: 6 }).map((_, index) => (
                  <Skeleton className="h-16 w-full" key={index} />
                ))}
              </div>
            ) : filteredGears.length === 0 ? (
              <div className="p-4">
                <EmptyState
                  description={filter.trim() === "" ? "Devices will appear here as soon as they are registered." : "No devices on this page match the current filter."}
                  title="No matching devices"
                />
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Device</TableHead>
                    <TableHead>Public Key</TableHead>
                    <TableHead className="w-24 text-center">Role</TableHead>
                    <TableHead className="w-24 text-center">Status</TableHead>
                    <TableHead>Registration</TableHead>
                    <TableHead>Updated</TableHead>
                    <TableHead>Flags</TableHead>
                    <TableHead className="text-right">Open</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredGears.map((gear) => (
                    <TableRow className="cursor-pointer hover:bg-muted/40" key={gear.public_key}>
                      <TableCell>
                        <Link className="font-medium hover:underline" to={`/devices/${encodeURIComponent(gear.public_key)}`}>
                          {deviceTitle(undefined, gear.public_key)}
                        </Link>
                      </TableCell>
                      <TableCell className="max-w-[16rem]">
                        <div className="truncate font-mono text-xs text-muted-foreground">{gear.public_key}</div>
                      </TableCell>
                      <TableCell className="text-center">
                        <div className="flex justify-center">
                          <Badge variant="outline">{gear.role}</Badge>
                        </div>
                      </TableCell>
                      <TableCell className="text-center">
                        <div className="flex justify-center">
                          <StatusBadge status={gear.status} />
                        </div>
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">{gear.auto_registered ? "Auto" : "Manual"}</TableCell>
                      <TableCell className="text-sm text-muted-foreground">{formatDate(gear.updated_at)}</TableCell>
                      <TableCell>
                        {gear.auto_registered ? <Badge variant="secondary">Auto</Badge> : <span className="text-sm text-muted-foreground">Manual</span>}
                      </TableCell>
                      <TableCell className="text-right text-muted-foreground">
                        <Link to={`/devices/${encodeURIComponent(gear.public_key)}`}>
                          <ChevronRight className="ml-auto size-4" />
                        </Link>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <div className="flex items-center justify-between border-t px-4 py-4 text-sm text-muted-foreground">
              <span>
                Showing {filteredGears.length} of {dashboard.gears.length} devices on page {devicePageNumber}
              </span>
              <span>{deviceList.hasNext ? "Next page available" : "End of results"}</span>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
