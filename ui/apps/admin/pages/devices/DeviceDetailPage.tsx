import type { ComponentType } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Activity, Ban, Check, ChevronLeft, Database, RefreshCw, Save, Server, Trash2 } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router-dom";

import { expectData, toMessage } from "../../../../packages/components/api";
import { Badge } from "../../../../packages/components/badge";
import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../../packages/components/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../../../packages/components/select";
import { Skeleton } from "../../../../packages/components/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../../../../packages/components/tabs";

import {
  approveGear,
  blockGear,
  deleteGear,
  putGearConfig,
  refreshGear,
  type Configuration,
  type GearRole,
} from "../../../../packages/adminservice";

import { DetailBlock } from "../../components/detail-block";
import { ErrorBanner, NoticeBanner } from "../../components/banners";
import { EmptyState } from "../../components/empty-state";
import { FormField } from "../../components/form-field";
import { PageBreadcrumb } from "../../components/page-breadcrumb";
import { StatusBadge } from "../../components/status-badge";
import { useDeviceDetail } from "../../hooks/useDeviceDetail";
import { deviceTitle, formatDate, formatShortKey } from "../../lib/format";

const FIRMWARE_CHANNEL_OPTIONS = ["stable", "beta", "testing"] as const;

export function DeviceDetailPage(): JSX.Element {
  const params = useParams();
  const navigate = useNavigate();
  const rawKey = params.publicKey ?? "";
  const publicKey = useMemo(() => {
    try {
      return decodeURIComponent(rawKey);
    } catch {
      return rawKey;
    }
  }, [rawKey]);

  const detail = useDeviceDetail(publicKey === "" ? undefined : publicKey);
  const [deviceNotice, setDeviceNotice] = useState<{ message: string; tone: "error" | "success" } | null>(null);
  const [deviceActionBusy, setDeviceActionBusy] = useState<string | null>(null);
  const [approveRole, setApproveRole] = useState<GearRole>("device");
  const [configChannel, setConfigChannel] = useState("stable");

  const registration = detail.data?.registration ?? null;
  const isBlocked = registration?.status === "blocked";

  useEffect(() => {
    const nextChannel = detail.data?.config?.firmware?.channel ?? "stable";
    setConfigChannel(FIRMWARE_CHANNEL_OPTIONS.includes(nextChannel as (typeof FIRMWARE_CHANNEL_OPTIONS)[number]) ? nextChannel : "stable");
    if (detail.data?.registration?.role && detail.data.registration.role !== "unspecified") {
      setApproveRole(detail.data.registration.role);
    }
  }, [detail.data?.config?.firmware?.channel, detail.data?.registration?.role]);

  const runDeviceAction = useCallback(async (name: string, action: () => Promise<void>, successMessage: string) => {
    setDeviceActionBusy(name);
    setDeviceNotice(null);
    try {
      await action();
      setDeviceNotice({ message: successMessage, tone: "success" });
    } catch (error) {
      setDeviceNotice({ message: toMessage(error), tone: "error" });
    } finally {
      setDeviceActionBusy(null);
    }
  }, []);

  const handleApprove = useCallback(async () => {
    if (publicKey === "") {
      return;
    }
    const nextRole: GearRole =
      detail.data?.registration?.role && detail.data.registration.role !== "unspecified" ? detail.data.registration.role : approveRole;
    await runDeviceAction(
      isBlocked ? "unblock" : "approve",
      async () => {
        await expectData(
          approveGear({
            body: { role: nextRole },
            path: { publicKey },
          }),
        );
        await detail.reload();
      },
      isBlocked ? `Device restored as ${nextRole}.` : `Device approved as ${nextRole}.`,
    );
  }, [approveRole, detail, isBlocked, publicKey, runDeviceAction]);

  const handleBlock = useCallback(async () => {
    if (publicKey === "") {
      return;
    }
    await runDeviceAction(
      "block",
      async () => {
        await expectData(blockGear({ path: { publicKey } }));
        await detail.reload();
      },
      "Device blocked.",
    );
  }, [detail, publicKey, runDeviceAction]);

  const handleRefreshDevice = useCallback(async () => {
    if (publicKey === "") {
      return;
    }
    await runDeviceAction(
      "refresh",
      async () => {
        await expectData(refreshGear({ path: { publicKey } }));
        await detail.reload();
      },
      "Device refreshed.",
    );
  }, [detail, publicKey, runDeviceAction]);

  const handleDeleteDevice = useCallback(async () => {
    if (publicKey === "") {
      return;
    }
    await runDeviceAction(
      "delete",
      async () => {
        await expectData(deleteGear({ path: { publicKey } }));
        navigate("/devices");
      },
      "Device deleted.",
    );
  }, [navigate, publicKey, runDeviceAction]);

  const handleSaveChannel = useCallback(async () => {
    if (publicKey === "") {
      return;
    }
    await runDeviceAction(
      "config",
      async () => {
        const nextConfig: Configuration = {
          ...(detail.data?.config ?? {}),
          firmware: {
            ...(detail.data?.config?.firmware ?? {}),
            channel: configChannel,
          },
        };
        await expectData(
          putGearConfig({
            body: nextConfig,
            path: { publicKey },
          }),
        );
        await detail.reload();
      },
      `Desired channel updated to ${configChannel}.`,
    );
  }, [configChannel, detail.data?.config, detail, publicKey, runDeviceAction]);

  if (publicKey === "") {
    return <EmptyState description="Missing device public key in the URL." title="Invalid route" />;
  }

  return (
    <div className="space-y-6">
      <PageBreadcrumb
        items={[
          { href: "/overview", label: "Overview" },
          { href: "/devices", label: "Devices" },
          { label: formatShortKey(publicKey) },
        ]}
      />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Devices</div>
          <h1 className="text-3xl font-semibold tracking-tight">{registration ? deviceTitle(detail.data?.info, registration.public_key) : "Device"}</h1>
          <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base break-all">{publicKey}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button asChild size="sm" variant="outline">
            <Link to="/devices">
              <ChevronLeft className="size-4" />
              Back to list
            </Link>
          </Button>
          <Button className="min-w-fit shrink-0 whitespace-nowrap" onClick={() => void detail.reload()} size="sm" variant="outline">
            <span className="inline-flex items-center gap-2 whitespace-nowrap">
              <RefreshCw className="size-4" />
              Reload
            </span>
          </Button>
          {registration ? <StatusBadge status={registration.status} /> : null}
        </div>
      </div>

      {detail.loading ? (
        <div className="space-y-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      ) : detail.error !== "" ? (
        <ErrorBanner message={detail.error} />
      ) : registration === null ? (
        <EmptyState description="This device could not be loaded." title="Not found" />
      ) : (
        <div className="space-y-4">
          {deviceNotice !== null ? <NoticeBanner message={deviceNotice.message} tone={deviceNotice.tone} /> : null}

          <div className="grid gap-4 xl:grid-cols-[1.2fr_0.8fr]">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Device Actions</CardTitle>
                <CardDescription>Approve, restore, block, refresh, or remove this device registration.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <FormField
                  description={
                    isBlocked
                      ? "Blocked devices can be restored back to service with their assigned role."
                      : "Choose the role to assign when this device moves into service."
                  }
                  label={isBlocked ? "Restore role" : "Approval role"}
                >
                  <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
                    <Select onValueChange={(value) => setApproveRole(value as GearRole)} value={approveRole}>
                      <SelectTrigger id="approve-role">
                        <SelectValue placeholder="Select role" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="device">device</SelectItem>
                        <SelectItem value="peer">peer</SelectItem>
                        <SelectItem value="admin">admin</SelectItem>
                      </SelectContent>
                    </Select>
                    <Button className="w-full md:w-auto" disabled={deviceActionBusy !== null} onClick={() => void handleApprove()} type="button">
                      <Check className="size-4" />
                      {isBlocked ? "Unblock" : "Approve"}
                    </Button>
                  </div>
                </FormField>

                <div className="space-y-3 rounded-lg border bg-muted/20 p-4">
                  <div className="space-y-1">
                    <div className="text-sm font-medium">Operational actions</div>
                    <p className="text-sm leading-6 text-muted-foreground">
                      Pull the latest state from the device, suspend it, or remove the registration entirely.
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button disabled={deviceActionBusy !== null} onClick={() => void handleRefreshDevice()} type="button" variant="outline">
                      <RefreshCw className="size-4" />
                      Refresh
                    </Button>
                    <Button disabled={deviceActionBusy !== null || isBlocked} onClick={() => void handleBlock()} type="button" variant="outline">
                      <Ban className="size-4" />
                      Block
                    </Button>
                    <Button disabled={deviceActionBusy !== null} onClick={() => void handleDeleteDevice()} type="button" variant="outline">
                      <Trash2 className="size-4" />
                      Delete
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Firmware Policy</CardTitle>
                <CardDescription>Set the desired firmware channel for this device.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <FormField description="This controls which release stream the device should follow." label="Desired channel">
                  <Select onValueChange={setConfigChannel} value={configChannel}>
                    <SelectTrigger id="device-channel">
                      <SelectValue placeholder="Select channel" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="stable">stable</SelectItem>
                      <SelectItem value="beta">beta</SelectItem>
                      <SelectItem value="testing">testing</SelectItem>
                    </SelectContent>
                  </Select>
                </FormField>
                <div className="flex justify-end border-t pt-4">
                  <Button disabled={deviceActionBusy !== null} onClick={() => void handleSaveChannel()} type="button">
                    <Save className="size-4" />
                    Save Channel
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>

          <div className="flex flex-col gap-3 rounded-xl border bg-muted/30 p-4 lg:flex-row lg:items-center lg:justify-between">
            <div className="space-y-1">
              <div className="text-base font-semibold">{deviceTitle(detail.data?.info, registration.public_key)}</div>
              <div className="text-sm text-muted-foreground break-all">{registration.public_key}</div>
            </div>
            <div className="flex flex-wrap gap-2">
              <Badge variant="outline">{registration.role}</Badge>
              {registration.auto_registered ? <Badge variant="secondary">Auto Registered</Badge> : null}
              {detail.data?.runtime?.online ? <Badge variant="success">Online</Badge> : <Badge variant="outline">Offline</Badge>}
            </div>
          </div>

          <Tabs className="space-y-4" defaultValue="info">
            <TabsList className="grid h-auto w-full grid-cols-5">
              <TabsTrigger value="info">Info</TabsTrigger>
              <TabsTrigger value="config">Config</TabsTrigger>
              <TabsTrigger value="runtime">Runtime</TabsTrigger>
              <TabsTrigger value="ota">OTA</TabsTrigger>
              <TabsTrigger value="raw">Raw</TabsTrigger>
            </TabsList>

            <TabsContent value="info">
              <div className="grid gap-4 lg:grid-cols-2">
                <DetailBlock
                  items={[
                    ["Name", detail.data?.info?.name],
                    ["Serial", detail.data?.info?.sn],
                    ["Manufacturer", detail.data?.info?.hardware?.manufacturer],
                    ["Model", detail.data?.info?.hardware?.model],
                    ["Revision", detail.data?.info?.hardware?.hardware_revision],
                  ]}
                  title="Device Info"
                />
                <DetailBlock
                  items={[
                    ["Created", registration.created_at],
                    ["Approved", registration.approved_at],
                    ["Updated", registration.updated_at],
                    ["Role", registration.role],
                    ["Status", registration.status],
                  ]}
                  title="Registration"
                />
              </div>
            </TabsContent>

            <TabsContent value="config">
              <div className="grid gap-4 lg:grid-cols-2">
                <DetailBlock
                  items={[
                    ["Channel", detail.data?.config?.firmware?.channel],
                    ["Depot", detail.data?.info?.hardware?.depot],
                    ["Firmware", detail.data?.info?.hardware?.firmware_semver],
                    ["Certifications", String(detail.data?.config?.certifications?.length ?? 0)],
                  ]}
                  title="Configuration"
                />
                <Card>
                  <CardHeader className="pb-3">
                    <CardTitle className="text-base">Certifications</CardTitle>
                    <CardDescription>Attached compliance metadata.</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-2">
                    {detail.data?.config?.certifications?.length ? (
                      detail.data.config.certifications.map((certification, index) => (
                        <div className="rounded-lg border bg-background px-3 py-2 text-sm" key={`${certification.id ?? "cert"}-${index}`}>
                          <div className="font-medium">{certification.id ?? "Unknown ID"}</div>
                          <div className="text-muted-foreground">
                            {certification.type ?? "type"} • {certification.authority_name ?? certification.authority ?? "authority"}
                          </div>
                        </div>
                      ))
                    ) : (
                      <EmptyState description="No certifications are attached to this device yet." title="No certifications" />
                    )}
                  </CardContent>
                </Card>
              </div>
            </TabsContent>

            <TabsContent value="runtime">
              <div className="grid gap-4 md:grid-cols-3">
                <RuntimeMetric icon={Activity} label="Online" value={detail.data?.runtime?.online ? "Yes" : "No"} />
                <RuntimeMetric icon={Server} label="Last Seen" value={formatDate(detail.data?.runtime?.last_seen_at)} />
                <RuntimeMetric icon={Database} label="Last Address" value={detail.data?.runtime?.last_addr ?? "—"} />
              </div>
            </TabsContent>

            <TabsContent value="ota">
              <Card>
                <CardContent className="pt-6">
                  <pre className="overflow-x-auto rounded-lg border bg-muted/50 p-4 text-xs leading-6 text-foreground">
                    {JSON.stringify(detail.data?.ota ?? null, null, 2)}
                  </pre>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="raw">
              <Card>
                <CardContent className="pt-6">
                  <pre className="overflow-x-auto rounded-lg border bg-muted/50 p-4 text-xs leading-6 text-foreground">
                    {JSON.stringify(detail.data, null, 2)}
                  </pre>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      )}
    </div>
  );
}

function RuntimeMetric({
  icon: Icon,
  label,
  value,
}: {
  icon: ComponentType<{ className?: string }>;
  label: string;
  value: string;
}): JSX.Element {
  return (
    <Card className="shadow-sm">
      <CardHeader className="gap-3 pb-4">
        <div className="flex size-10 items-center justify-center rounded-lg border bg-primary/5 text-primary">
          <Icon className="size-4" />
        </div>
        <div className="space-y-1">
          <CardDescription>{label}</CardDescription>
          <CardTitle className="text-base">{value}</CardTitle>
        </div>
      </CardHeader>
    </Card>
  );
}
