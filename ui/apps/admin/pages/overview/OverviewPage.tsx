import type { ComponentType } from "react";
import { AudioLines, Boxes, ChevronRight, FolderKanban, HardDrive, KeyRound, Mic2, Server, ShieldCheck, Workflow } from "lucide-react";
import { Link } from "react-router-dom";

import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../../packages/components/card";
import { Skeleton } from "../../../../packages/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../../packages/components/table";

import { EmptyState } from "../../../../packages/components/empty-state";
import { StatusBadge } from "../../../../packages/components/status-badge";
import { useOverviewData } from "../../hooks/useOverviewData";
import { formatRelease, formatServerTime, formatShortKey } from "../../lib/format";

export function OverviewPage(): JSX.Element {
  const dashboard = useOverviewData();
  const latestDevices = dashboard.gears.slice(0, 5);
  const latestDepots = dashboard.depots.slice(0, 4);
  const autoCount = dashboard.gears.filter((gear) => gear.auto_registered).length;

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Overview</div>
        <h1 className="text-3xl font-semibold tracking-tight">Dashboard</h1>
        <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">
          Server health, a snapshot of devices on the first page, and firmware depots.
        </p>
      </div>

      {dashboard.error !== "" ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">{dashboard.error}</div>
      ) : null}

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          description={formatShortKey(dashboard.serverInfo?.public_key)}
          icon={Server}
          label="Server Build"
          value={dashboard.serverInfo?.build_commit ?? "dev"}
        />
        <MetricCard
          description="First page snapshot"
          icon={Boxes}
          label="Devices This Page"
          value={String(dashboard.gears.length)}
        />
        <MetricCard
          description={`${dashboard.gears.length - autoCount} manual or approved on this page`}
          icon={ShieldCheck}
          label="Auto Registered"
          value={String(autoCount)}
        />
        <MetricCard
          description={formatServerTime(dashboard.serverInfo?.server_time)}
          icon={HardDrive}
          label="Firmware Depots"
          value={String(dashboard.depots.length)}
        />
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <Card className="border-border/60 bg-background/90 shadow-sm">
          <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
            <div className="space-y-1">
              <CardTitle>Recent Devices</CardTitle>
              <CardDescription>Latest devices from the first page of results.</CardDescription>
            </div>
            <Button asChild size="sm" variant="outline">
              <Link to="/devices">
                Open Devices
                <ChevronRight className="size-4" />
              </Link>
            </Button>
          </CardHeader>
          <CardContent className="space-y-3">
            {dashboard.loading ? (
              <div className="space-y-3">
                {Array.from({ length: 4 }).map((_, index) => (
                  <Skeleton className="h-16 w-full" key={index} />
                ))}
              </div>
            ) : latestDevices.length === 0 ? (
              <EmptyState
                description="Register devices and they will show up here as clickable detail entries."
                title="No devices yet"
              />
            ) : (
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Device</TableHead>
                      <TableHead>Role</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="text-right">Open</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {latestDevices.map((gear) => (
                      <TableRow className="cursor-pointer" key={gear.public_key}>
                        <TableCell className="font-medium">
                          <Link className="hover:underline" to={`/devices/${encodeURIComponent(gear.public_key)}`}>
                            {formatShortKey(gear.public_key)}
                          </Link>
                        </TableCell>
                        <TableCell>{gear.role}</TableCell>
                        <TableCell>
                          <StatusBadge status={gear.status} />
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
              </div>
            )}
          </CardContent>
        </Card>

        <Card className="border-border/60 bg-background/90 shadow-sm">
          <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
            <div className="space-y-1">
              <CardTitle>Firmware Snapshot</CardTitle>
              <CardDescription>A quick summary before opening depot details.</CardDescription>
            </div>
            <Button asChild size="sm" variant="outline">
              <Link to="/firmware">
                Open Firmware
                <ChevronRight className="size-4" />
              </Link>
            </Button>
          </CardHeader>
          <CardContent className="space-y-3">
            {dashboard.loading ? (
              <div className="space-y-3">
                {Array.from({ length: 4 }).map((_, index) => (
                  <Skeleton className="h-14 w-full" key={index} />
                ))}
              </div>
            ) : latestDepots.length === 0 ? (
              <EmptyState
                description="Once depot data exists, stable and testing versions will show here."
                title="No depots yet"
              />
            ) : (
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Depot</TableHead>
                      <TableHead>Stable</TableHead>
                      <TableHead>Testing</TableHead>
                      <TableHead className="text-right">Files</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {latestDepots.map((depot) => (
                      <TableRow key={depot.name}>
                        <TableCell className="font-medium">
                          <Link className="hover:underline" to={`/firmware/${encodeURIComponent(depot.name)}`}>
                            {depot.name}
                          </Link>
                        </TableCell>
                        <TableCell>{formatRelease(depot.stable)}</TableCell>
                        <TableCell>{formatRelease(depot.testing)}</TableCell>
                        <TableCell className="text-right">{depot.info?.files?.length ?? 0}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      </section>

      <Card className="border-border/60 bg-background/90 shadow-sm">
        <CardHeader>
          <CardTitle>Shortcuts</CardTitle>
          <CardDescription>Jump to primary admin surfaces.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Button asChild variant="outline">
            <Link to="/devices">
              <Boxes className="size-4" />
              Devices
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link to="/firmware">
              <HardDrive className="size-4" />
              Firmware
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link to="/providers/credentials">
              <KeyRound className="size-4" />
              Credentials
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link to="/providers/minimax-tenants">
              <AudioLines className="size-4" />
              MiniMax Tenants
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link to="/ai/voices">
              <Mic2 className="size-4" />
              Voices
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link to="/ai/workspace-templates">
              <Workflow className="size-4" />
              Workspace Templates
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link to="/ai/workspaces">
              <FolderKanban className="size-4" />
              Workspaces
            </Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function MetricCard({
  description,
  icon: Icon,
  label,
  value,
}: {
  description: string;
  icon: ComponentType<{ className?: string }>;
  label: string;
  value: string;
}): JSX.Element {
  return (
    <Card className="border-border/60 bg-background/90 shadow-sm">
      <CardHeader className="space-y-3">
        <div className="flex items-center justify-between">
          <CardDescription>{label}</CardDescription>
          <div className="rounded-lg border bg-primary/5 p-2 text-primary">
            <Icon className="size-4" />
          </div>
        </div>
        <div className="space-y-1">
          <CardTitle className="text-2xl">{value}</CardTitle>
          <div className="text-sm text-muted-foreground">{description}</div>
        </div>
      </CardHeader>
    </Card>
  );
}
