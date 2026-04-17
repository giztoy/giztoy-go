import type { ComponentType, JSX, ReactNode } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Activity,
  Ban,
  Boxes,
  ChevronRight,
  Check,
  Database,
  HardDrive,
  LayoutDashboard,
  MemoryStick,
  Package,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
  Server,
  ShieldCheck,
  Trash2,
  Upload,
} from "lucide-react";

import { Badge } from "../../packages/components/badge";
import { Button } from "../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../packages/components/card";
import { expectData, toMessage } from "../../packages/components/api";
import { Input } from "../../packages/components/input";
import { Label } from "../../packages/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../packages/components/select";
import { Skeleton } from "../../packages/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../packages/components/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../../packages/components/tabs";
import { cn } from "../../packages/components/utils";
import {
  listDepots,
  putChannel,
  putDepotInfo,
  releaseDepot,
  rollbackDepot,
  type Depot,
  type DepotInfo as AdminDepotInfo,
  type DepotRelease,
} from "../../packages/adminservice";
import { client as adminClient } from "../../packages/adminservice/client.gen";
import {
  approveGear,
  blockGear,
  deleteGear,
  getGear,
  getGearConfig,
  getGearInfo,
  getGearRuntime,
  listGears,
  putGearConfig,
  refreshGear,
  type Configuration,
  type DeviceInfo,
  type GearRole,
  type Registration,
  type RegistrationList,
  type Runtime,
} from "../../packages/gearservice";
import { client as gearClient } from "../../packages/gearservice/client.gen";
import { getServerInfo, type ServerInfo } from "../../packages/serverpublic";
import { client as publicClient } from "../../packages/serverpublic/client.gen";

type Section = "overview" | "devices" | "firmware" | "memory";

interface RouteState {
  publicKey?: string;
  section: Section;
}

interface DashboardState {
  depots: Depot[];
  error: string;
  gears: Registration[];
  loading: boolean;
  serverInfo: ServerInfo | null;
}

interface GearDetail {
  config: Configuration | null;
  info: DeviceInfo | null;
  registration: Registration | null;
  runtime: Runtime | null;
}

interface GearDetailState {
  data: GearDetail | null;
  error: string;
  loading: boolean;
}

interface NoticeState {
  message: string;
  tone: "error" | "success";
}

adminClient.setConfig({
  baseUrl: "/api/admin",
  responseStyle: "fields",
  throwOnError: false,
});

gearClient.setConfig({
  baseUrl: "/api/gear",
  responseStyle: "fields",
  throwOnError: false,
});

publicClient.setConfig({
  baseUrl: "/api/public",
  responseStyle: "fields",
  throwOnError: false,
});

function App(): JSX.Element {
  const [route, navigate] = useHashRoute();
  const [filter, setFilter] = useState("");
  const [dashboard, setDashboard] = useState<DashboardState>({
    depots: [],
    error: "",
    gears: [],
    loading: true,
    serverInfo: null,
  });
  const [detailState, setDetailState] = useState<GearDetailState>({
    data: null,
    error: "",
    loading: false,
  });
  const [deviceActionBusy, setDeviceActionBusy] = useState<string | null>(null);
  const [deviceNotice, setDeviceNotice] = useState<NoticeState | null>(null);
  const [approveRole, setApproveRole] = useState<GearRole>("device");
  const [configChannel, setConfigChannel] = useState("stable");
  const [firmwareBusy, setFirmwareBusy] = useState<string | null>(null);
  const [firmwareNotice, setFirmwareNotice] = useState<NoticeState | null>(null);
  const [uploadDepot, setUploadDepot] = useState("");
  const [uploadChannel, setUploadChannel] = useState("testing");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [infoDepot, setInfoDepot] = useState("");
  const [infoFile, setInfoFile] = useState<File | null>(null);

  const refreshDashboard = useCallback(async () => {
    setDashboard((current) => ({ ...current, error: "", loading: true }));
    try {
      const [serverInfo, registrations, depots] = await Promise.all([
        expectData(getServerInfo()),
        expectData(listGears()),
        expectData(listDepots()),
      ]);

      setDashboard({
        depots: depots.items ?? [],
        error: "",
        gears: registrations.items ?? [],
        loading: false,
        serverInfo,
      });
    } catch (error) {
      setDashboard((current) => ({
        ...current,
        error: toMessage(error),
        loading: false,
      }));
    }
  }, []);

  useEffect(() => {
    void refreshDashboard();
  }, [refreshDashboard]);

  useEffect(() => {
    if (route.section !== "devices" || route.publicKey === undefined) {
      setDetailState({
        data: null,
        error: "",
        loading: false,
      });
      return;
    }

    let cancelled = false;
    setDetailState({
      data: {
        config: null,
        info: null,
        registration: dashboard.gears.find((gear) => gear.public_key === route.publicKey) ?? null,
        runtime: null,
      },
      error: "",
      loading: true,
    });

    const loadDetail = async () => {
      try {
        const [registration, info, config, runtime] = await Promise.all([
          expectData(getGear({ path: { publicKey: route.publicKey ?? "" } })),
          expectData(getGearInfo({ path: { publicKey: route.publicKey ?? "" } })),
          expectData(getGearConfig({ path: { publicKey: route.publicKey ?? "" } })),
          expectData(getGearRuntime({ path: { publicKey: route.publicKey ?? "" } })),
        ]);

        if (cancelled) {
          return;
        }

        setDetailState({
          data: {
            config,
            info,
            registration,
            runtime,
          },
          error: "",
          loading: false,
        });
      } catch (error) {
        if (cancelled) {
          return;
        }
        setDetailState({
          data: {
            config: null,
            info: null,
            registration: dashboard.gears.find((gear) => gear.public_key === route.publicKey) ?? null,
            runtime: null,
          },
          error: toMessage(error),
          loading: false,
        });
      }
    };

    void loadDetail();

    return () => {
      cancelled = true;
    };
  }, [dashboard.gears, route]);

  useEffect(() => {
    if (window.location.hash === "") {
      navigate({ section: "overview" });
    }
  }, [navigate]);

  useEffect(() => {
    if (route.section !== "devices") {
      setDeviceNotice(null);
      return;
    }
    setConfigChannel(detailState.data?.config?.firmware?.channel ?? "stable");
    if (detailState.data?.registration?.role && detailState.data.registration.role !== "unspecified") {
      setApproveRole(detailState.data.registration.role);
    }
  }, [detailState.data?.config?.firmware?.channel, detailState.data?.registration?.role, route.section]);

  const filteredGears = useMemo(() => {
    if (filter.trim() === "") {
      return dashboard.gears;
    }
    const query = filter.trim().toLowerCase();
    return dashboard.gears.filter((gear) =>
      [gear.public_key, gear.role, gear.status, gear.auto_registered ? "auto" : "manual"].some((value) =>
        value.toLowerCase().includes(query),
      ),
    );
  }, [dashboard.gears, filter]);

  const selectedRegistration = useMemo(
    () =>
      detailState.data?.registration ??
      dashboard.gears.find((gear) => gear.public_key === route.publicKey) ??
      null,
    [dashboard.gears, detailState.data, route.publicKey],
  );
  const isBlocked = selectedRegistration?.status === "blocked";

  const activeCount = dashboard.gears.filter((gear) => gear.status === "active").length;
  const autoCount = dashboard.gears.filter((gear) => gear.auto_registered).length;
  const latestDevices = dashboard.gears.slice(0, 5);
  const latestDepots = dashboard.depots.slice(0, 4);
  const page = sectionMeta(route);

  const runDeviceAction = useCallback(
    async (name: string, action: () => Promise<void>, successMessage: string) => {
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
    },
    [],
  );

  const handleApprove = useCallback(async () => {
    if (route.publicKey === undefined) {
      return;
    }
    const publicKey = route.publicKey;
    const nextRole: GearRole = (
      detailState.data?.registration?.role && detailState.data.registration.role !== "unspecified"
        ? detailState.data.registration.role
        : approveRole
    );
    await runDeviceAction(
      isBlocked ? "unblock" : "approve",
      async () => {
        await expectData(
          approveGear({
            body: { role: nextRole },
            path: { publicKey },
          }),
        );
        await refreshDashboard();
      },
      isBlocked ? `Device restored as ${nextRole}.` : `Device approved as ${nextRole}.`,
    );
  }, [approveRole, detailState.data?.registration?.role, isBlocked, refreshDashboard, route.publicKey, runDeviceAction]);

  const handleBlock = useCallback(async () => {
    if (route.publicKey === undefined) {
      return;
    }
    const publicKey = route.publicKey;
    await runDeviceAction(
      "block",
      async () => {
        await expectData(blockGear({ path: { publicKey } }));
        await refreshDashboard();
      },
      "Device blocked.",
    );
  }, [refreshDashboard, route.publicKey, runDeviceAction]);

  const handleRefreshDevice = useCallback(async () => {
    if (route.publicKey === undefined) {
      return;
    }
    const publicKey = route.publicKey;
    await runDeviceAction(
      "refresh",
      async () => {
        await expectData(refreshGear({ path: { publicKey } }));
        await refreshDashboard();
      },
      "Device refreshed.",
    );
  }, [refreshDashboard, route.publicKey, runDeviceAction]);

  const handleDeleteDevice = useCallback(async () => {
    if (route.publicKey === undefined) {
      return;
    }
    const publicKey = route.publicKey;
    await runDeviceAction(
      "delete",
      async () => {
        await expectData(deleteGear({ path: { publicKey } }));
        navigate({ section: "devices" });
        await refreshDashboard();
      },
      "Device deleted.",
    );
  }, [navigate, refreshDashboard, route.publicKey, runDeviceAction]);

  const handleSaveChannel = useCallback(async () => {
    if (route.publicKey === undefined) {
      return;
    }
    const publicKey = route.publicKey;
    await runDeviceAction(
      "config",
      async () => {
        const nextConfig: Configuration = {
          ...(detailState.data?.config ?? {}),
          firmware: {
            ...(detailState.data?.config?.firmware ?? {}),
            channel: configChannel,
          },
        };
        await expectData(
          putGearConfig({
            body: nextConfig,
            path: { publicKey },
          }),
        );
        await refreshDashboard();
      },
      `Desired channel updated to ${configChannel}.`,
    );
  }, [configChannel, detailState.data?.config, refreshDashboard, route.publicKey, runDeviceAction]);

  const handleUploadFirmware = useCallback(async () => {
    if (uploadDepot.trim() === "" || uploadFile === null) {
      setFirmwareNotice({ message: "Select a depot and a firmware tarball first.", tone: "error" });
      return;
    }
    setFirmwareBusy("upload");
    setFirmwareNotice(null);
    try {
      await expectData(
        putChannel({
          body: uploadFile,
          path: { channel: uploadChannel, depot: uploadDepot.trim() },
        }),
      );
      await refreshDashboard();
      setFirmwareNotice({
        message: `Uploaded ${uploadFile.name} to ${uploadDepot.trim()}/${uploadChannel}.`,
        tone: "success",
      });
    } catch (error) {
      setFirmwareNotice({ message: toMessage(error), tone: "error" });
    } finally {
      setFirmwareBusy(null);
    }
  }, [refreshDashboard, uploadChannel, uploadDepot, uploadFile]);

  const handleUploadDepotInfo = useCallback(async () => {
    if (infoDepot.trim() === "" || infoFile === null) {
      setFirmwareNotice({ message: "Select a depot and an info.json file first.", tone: "error" });
      return;
    }
    setFirmwareBusy("info");
    setFirmwareNotice(null);
    try {
      const text = await infoFile.text();
      const parsed = JSON.parse(text) as AdminDepotInfo;
      await expectData(
        putDepotInfo({
          body: parsed,
          path: { depot: infoDepot.trim() },
        }),
      );
      await refreshDashboard();
      setFirmwareNotice({
        message: `Updated metadata for depot ${infoDepot.trim()}.`,
        tone: "success",
      });
    } catch (error) {
      setFirmwareNotice({ message: toMessage(error), tone: "error" });
    } finally {
      setFirmwareBusy(null);
    }
  }, [infoDepot, infoFile, refreshDashboard]);

  const handleDepotAction = useCallback(
    async (depot: string, action: "release" | "rollback") => {
      setFirmwareBusy(`${action}:${depot}`);
      setFirmwareNotice(null);
      try {
        if (action === "release") {
          await expectData(releaseDepot({ path: { depot } }));
        } else {
          await expectData(rollbackDepot({ path: { depot } }));
        }
        await refreshDashboard();
        setFirmwareNotice({
          message: action === "release" ? `Released depot ${depot}.` : `Rolled back depot ${depot}.`,
          tone: "success",
        });
      } catch (error) {
        setFirmwareNotice({ message: toMessage(error), tone: "error" });
      } finally {
        setFirmwareBusy(null);
      }
    },
    [refreshDashboard],
  );

  return (
    <div className="min-h-screen bg-muted/30">
      <div className="grid min-h-screen lg:grid-cols-[280px_minmax(0,1fr)]">
        <aside className="border-r border-border/60 bg-slate-950 text-slate-100">
          <div className="flex h-full flex-col gap-8 p-4 lg:p-6">
            <div className="flex items-center gap-3">
              <div className="flex size-11 items-center justify-center rounded-xl bg-primary font-semibold text-primary-foreground shadow">
                GC
              </div>
              <div>
                <div className="font-semibold tracking-tight">GizClaw Admin</div>
                <div className="text-sm text-slate-400">Operations console</div>
              </div>
            </div>

            <div className="space-y-2">
              <div className="px-2 text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">Navigation</div>
              <SidebarButton
                active={route.section === "overview"}
                description="Server health, recent devices, and rollout summary"
                icon={LayoutDashboard}
                label="Overview"
                onClick={() => navigate({ section: "overview" })}
              />
              <SidebarButton
                active={route.section === "devices"}
                description="Search devices and open a dedicated detail view"
                icon={Boxes}
                label="Devices"
                onClick={() => navigate({ section: "devices" })}
              />
              <SidebarButton
                active={route.section === "firmware"}
                description="Inspect depot snapshots and release channels"
                icon={HardDrive}
                label="Firmware"
                onClick={() => navigate({ section: "firmware" })}
              />
              <SidebarButton
                active={route.section === "memory"}
                description="Reserved shell space for memory and store tooling"
                icon={MemoryStick}
                label="Memory"
                onClick={() => navigate({ section: "memory" })}
              />
            </div>

            <Card className="border-slate-800 bg-slate-900/70 text-slate-100 shadow-none">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">Live Summary</CardTitle>
                <CardDescription className="text-slate-400">Current server and inventory snapshot.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3 text-sm">
                <SidebarStat label="Build" value={dashboard.serverInfo?.build_commit ?? "dev"} />
                <SidebarStat label="Gears" value={String(dashboard.gears.length)} />
                <SidebarStat label="Active" value={String(activeCount)} />
                <SidebarStat label="Depots" value={String(dashboard.depots.length)} />
              </CardContent>
            </Card>

            <div className="mt-auto space-y-3 rounded-xl border border-slate-800 bg-slate-900/60 p-4 text-sm text-slate-400">
              <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Last updated</p>
              <p className="leading-6">{formatServerTime(dashboard.serverInfo?.server_time)}</p>
            </div>
          </div>
        </aside>

        <main className="min-w-0">
          <div className="mx-auto flex max-w-7xl flex-col gap-6 p-4 lg:p-8">
            <header className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div className="space-y-2">
                <div className="text-xs font-semibold uppercase tracking-[0.18em] text-primary">{page.eyebrow}</div>
                <div className="space-y-1">
                  <h1 className="text-3xl font-semibold tracking-tight text-foreground lg:text-4xl">{page.title}</h1>
                  <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">{page.description}</p>
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <Button onClick={() => void refreshDashboard()}>
                  <RefreshCw className="size-4" />
                  Refresh Data
                </Button>
                {route.section === "devices" && route.publicKey !== undefined ? (
                  <Button onClick={() => navigate({ publicKey: route.publicKey, section: "devices" })} variant="outline">
                    <Search className="size-4" />
                    Refresh Device
                  </Button>
                ) : null}
              </div>
            </header>

            {dashboard.error !== "" ? <ErrorBanner message={dashboard.error} /> : null}

            {route.section === "overview" ? (
              <div className="space-y-6">
                <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                  <MetricCard
                    description={formatShortKey(dashboard.serverInfo?.public_key)}
                    icon={Server}
                    label="Server Build"
                    value={dashboard.serverInfo?.build_commit ?? "dev"}
                  />
                  <MetricCard
                    description={`${activeCount} active`}
                    icon={Boxes}
                    label="Registered Devices"
                    value={String(dashboard.gears.length)}
                  />
                  <MetricCard
                    description={`${dashboard.gears.length - autoCount} manual or approved`}
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
                  <Card>
                    <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
                      <div className="space-y-1">
                        <CardTitle>Recent Devices</CardTitle>
                        <CardDescription>Open a device to review details, status, and recent state.</CardDescription>
                      </div>
                      <Button onClick={() => navigate({ section: "devices" })} size="sm" variant="outline">
                        Open Devices
                        <ChevronRight className="size-4" />
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
                        <div className="space-y-3">
                          {latestDevices.map((gear) => (
                            <button
                              className="flex w-full items-center justify-between rounded-lg border border-border bg-background p-4 text-left transition hover:bg-accent"
                              key={gear.public_key}
                              onClick={() => navigate({ publicKey: gear.public_key, section: "devices" })}
                              type="button"
                            >
                              <div className="space-y-1">
                                <div className="font-medium text-foreground">{formatShortKey(gear.public_key)}</div>
                                <div className="text-sm text-muted-foreground">
                                  {gear.role} • {gear.status}
                                </div>
                              </div>
                              <ChevronRight className="size-4 text-muted-foreground" />
                            </button>
                          ))}
                        </div>
                      )}
                    </CardContent>
                  </Card>

                  <Card>
                    <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
                      <div className="space-y-1">
                        <CardTitle>Firmware Snapshot</CardTitle>
                        <CardDescription>A quick summary before opening full depot details.</CardDescription>
                      </div>
                      <Button onClick={() => navigate({ section: "firmware" })} size="sm" variant="outline">
                        Open Firmware
                        <ChevronRight className="size-4" />
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
                        latestDepots.map((depot) => (
                          <div
                            className="flex items-center justify-between rounded-lg border border-border bg-background px-4 py-3"
                            key={depot.name}
                          >
                            <div className="space-y-1">
                              <div className="font-medium">{depot.name}</div>
                              <div className="text-sm text-muted-foreground">
                                stable {formatRelease(depot.stable)} • testing {formatRelease(depot.testing)}
                              </div>
                            </div>
                            <Badge variant="secondary">{`${depot.info?.files?.length ?? 0} files`}</Badge>
                          </div>
                        ))
                      )}
                    </CardContent>
                  </Card>
                </section>

                <section className="grid gap-6 lg:grid-cols-3">
                  <ModuleCard
                    description="Reserved for persistent admin insights like in-memory state and cache visibility."
                    icon={MemoryStick}
                    title="Memory"
                  />
                  <ModuleCard
                    description="Natural next home for job queues, OTA refresh history, or background workflows."
                    icon={Activity}
                    title="Jobs"
                  />
                  <ModuleCard
                    description="Dedicated operational diagnostics, event streams, and future admin telemetry."
                    icon={Database}
                    title="Diagnostics"
                  />
                </section>
              </div>
            ) : null}

            {route.section === "devices" ? (
              <div className="grid gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
                <Card className="overflow-hidden">
                  <CardHeader className="space-y-4">
                    <div className="space-y-1">
                      <CardTitle>Devices</CardTitle>
                      <CardDescription>Search the list, then open one device into a detailed operational view.</CardDescription>
                    </div>
                    <div className="relative">
                      <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        className="pl-9"
                        onChange={(event) => setFilter(event.target.value)}
                        placeholder="Search by key, role, or status"
                        value={filter}
                      />
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    {dashboard.loading ? (
                      Array.from({ length: 6 }).map((_, index) => <Skeleton className="h-16 w-full" key={index} />)
                    ) : filteredGears.length === 0 ? (
                      <EmptyState
                        description="Devices will appear here as soon as they are registered."
                        title="No matching devices"
                      />
                    ) : (
                      filteredGears.map((gear) => {
                        const active = route.publicKey === gear.public_key;
                        return (
                          <button
                            className={cn(
                              "flex w-full items-center justify-between rounded-xl border p-4 text-left transition",
                              active ? "border-primary bg-primary/5" : "border-border bg-background hover:bg-accent",
                            )}
                            key={gear.public_key}
                            onClick={() => navigate({ publicKey: gear.public_key, section: "devices" })}
                            type="button"
                          >
                            <div className="space-y-2">
                              <div className="font-medium">{formatShortKey(gear.public_key)}</div>
                              <div className="flex flex-wrap gap-2">
                                <StatusBadge status={gear.status} />
                                <Badge variant="outline">{gear.role}</Badge>
                                {gear.auto_registered ? <Badge variant="secondary">Auto</Badge> : null}
                              </div>
                            </div>
                            <ChevronRight className="size-4 text-muted-foreground" />
                          </button>
                        );
                      })
                    )}
                  </CardContent>
                </Card>

                <Card className="min-h-[32rem]">
                  <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
                    <div className="space-y-1">
                      <CardTitle>{selectedRegistration ? deviceTitle(detailState.data?.info, selectedRegistration.public_key) : "Select a Device"}</CardTitle>
                      <CardDescription>
                        {selectedRegistration
                          ? "A dedicated detail workspace for the selected device."
                          : "Pick any device from the list to inspect info, config, runtime, and raw JSON."}
                      </CardDescription>
                    </div>
                    {selectedRegistration ? <StatusBadge status={selectedRegistration.status} /> : null}
                  </CardHeader>
                  <CardContent>
                    {selectedRegistration === null ? (
                      <EmptyState
                        description="Select a device from the list to review its details, configuration, runtime, and raw data."
                        title="No device selected"
                      />
                    ) : detailState.loading ? (
                      <div className="space-y-4">
                        <Skeleton className="h-24 w-full" />
                        <Skeleton className="h-64 w-full" />
                      </div>
                    ) : (
                      <div className="space-y-4">
                        {detailState.error !== "" ? <ErrorBanner message={detailState.error} /> : null}
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
                              <FormField
                                description="This controls which release stream the device should follow."
                                label="Desired channel"
                              >
                                <Select onValueChange={setConfigChannel} value={configChannel}>
                                  <SelectTrigger id="device-channel">
                                    <SelectValue placeholder="Select channel" />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="rollback">rollback</SelectItem>
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
                            <div className="text-base font-semibold">{deviceTitle(detailState.data?.info, selectedRegistration.public_key)}</div>
                            <div className="text-sm text-muted-foreground break-all">{selectedRegistration.public_key}</div>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            <Badge variant="outline">{selectedRegistration.role}</Badge>
                            {selectedRegistration.auto_registered ? <Badge variant="secondary">Auto Registered</Badge> : null}
                            {detailState.data?.runtime?.online ? <Badge variant="success">Online</Badge> : <Badge variant="outline">Offline</Badge>}
                          </div>
                        </div>

                        <Tabs defaultValue="overview">
                          <TabsList className="grid w-full grid-cols-4">
                            <TabsTrigger value="overview">Overview</TabsTrigger>
                            <TabsTrigger value="config">Config</TabsTrigger>
                            <TabsTrigger value="runtime">Runtime</TabsTrigger>
                            <TabsTrigger value="raw">Raw JSON</TabsTrigger>
                          </TabsList>

                          <TabsContent value="overview">
                            <div className="grid gap-4 lg:grid-cols-2">
                              <DetailBlock
                                items={[
                                  ["Name", detailState.data?.info?.name],
                                  ["Serial", detailState.data?.info?.sn],
                                  ["Manufacturer", detailState.data?.info?.hardware?.manufacturer],
                                  ["Model", detailState.data?.info?.hardware?.model],
                                  ["Revision", detailState.data?.info?.hardware?.hardware_revision],
                                ]}
                                title="Device Info"
                              />
                              <DetailBlock
                                items={[
                                  ["Created", selectedRegistration.created_at],
                                  ["Approved", selectedRegistration.approved_at],
                                  ["Updated", selectedRegistration.updated_at],
                                  ["Role", selectedRegistration.role],
                                  ["Status", selectedRegistration.status],
                                ]}
                                title="Registration"
                              />
                            </div>
                          </TabsContent>

                          <TabsContent value="config">
                            <div className="grid gap-4 lg:grid-cols-2">
                              <DetailBlock
                                items={[
                                  ["Channel", detailState.data?.config?.firmware?.channel],
                                  ["Depot", detailState.data?.info?.hardware?.depot],
                                  ["Firmware", detailState.data?.info?.hardware?.firmware_semver],
                                  ["Certifications", String(detailState.data?.config?.certifications?.length ?? 0)],
                                ]}
                                title="Configuration"
                              />
                              <Card>
                                <CardHeader className="pb-3">
                                  <CardTitle className="text-base">Certifications</CardTitle>
                                  <CardDescription>Future device compliance and metadata can live here.</CardDescription>
                                </CardHeader>
                                <CardContent className="space-y-2">
                                  {detailState.data?.config?.certifications?.length ? (
                                    detailState.data.config.certifications.map((certification, index) => (
                                      <div className="rounded-lg border bg-background px-3 py-2 text-sm" key={`${certification.id ?? "cert"}-${index}`}>
                                        <div className="font-medium">{certification.id ?? "Unknown ID"}</div>
                                        <div className="text-muted-foreground">
                                          {certification.type ?? "type"} • {certification.authority_name ?? certification.authority ?? "authority"}
                                        </div>
                                      </div>
                                    ))
                                  ) : (
                                    <EmptyState
                                      description="No certifications are attached to this device yet."
                                      title="No certifications"
                                    />
                                  )}
                                </CardContent>
                              </Card>
                            </div>
                          </TabsContent>

                          <TabsContent value="runtime">
                            <div className="grid gap-4 md:grid-cols-3">
                              <RuntimeMetric
                                icon={Activity}
                                label="Online"
                                value={detailState.data?.runtime?.online ? "Yes" : "No"}
                              />
                              <RuntimeMetric
                                icon={Server}
                                label="Last Seen"
                                value={formatDate(detailState.data?.runtime?.last_seen_at)}
                              />
                              <RuntimeMetric
                                icon={Database}
                                label="Last Address"
                                value={detailState.data?.runtime?.last_addr ?? "—"}
                              />
                            </div>
                          </TabsContent>

                          <TabsContent value="raw">
                            <Card>
                              <CardContent className="pt-6">
                                <pre className="overflow-x-auto rounded-lg border bg-muted/50 p-4 text-xs leading-6 text-foreground">
                                  {JSON.stringify(detailState.data, null, 2)}
                                </pre>
                              </CardContent>
                            </Card>
                          </TabsContent>
                        </Tabs>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            ) : null}

            {route.section === "firmware" ? (
              <div className="space-y-6">
                {firmwareNotice !== null ? <NoticeBanner message={firmwareNotice.message} tone={firmwareNotice.tone} /> : null}

                <div className="grid gap-6 xl:grid-cols-2">
                  <Card>
                    <CardHeader>
                      <CardTitle>Upload Firmware</CardTitle>
                      <CardDescription>Upload a release tarball into a depot channel.</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div className="grid gap-4 md:grid-cols-2">
                        <FormField description="The firmware depot receiving this release." label="Depot">
                          <Input id="upload-depot" onChange={(event) => setUploadDepot(event.target.value)} placeholder="demo-main" value={uploadDepot} />
                        </FormField>
                        <FormField description="Choose which rollout lane this tarball should land in." label="Channel">
                          <Select onValueChange={setUploadChannel} value={uploadChannel}>
                            <SelectTrigger id="upload-channel">
                              <SelectValue placeholder="Select channel" />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="rollback">rollback</SelectItem>
                              <SelectItem value="stable">stable</SelectItem>
                              <SelectItem value="beta">beta</SelectItem>
                              <SelectItem value="testing">testing</SelectItem>
                            </SelectContent>
                          </Select>
                        </FormField>
                      </div>

                      <FormField description="Upload the release archive produced by your firmware build." label="Release tarball">
                        <Input
                          accept=".tar,.tgz,.tar.gz,application/octet-stream"
                          id="upload-file"
                          onChange={(event) => setUploadFile(event.target.files?.[0] ?? null)}
                          type="file"
                        />
                      </FormField>

                      <div className="flex justify-end border-t pt-4">
                        <Button disabled={firmwareBusy !== null} onClick={() => void handleUploadFirmware()} type="button">
                          <Upload className="size-4" />
                          Upload Release
                        </Button>
                      </div>
                    </CardContent>
                  </Card>

                  <Card>
                    <CardHeader>
                      <CardTitle>Update Depot Info</CardTitle>
                      <CardDescription>Upload an `info.json` manifest for a depot.</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <FormField description="The target depot whose file manifest should be updated." label="Depot">
                        <Input id="info-depot" onChange={(event) => setInfoDepot(event.target.value)} placeholder="demo-main" value={infoDepot} />
                      </FormField>

                      <FormField description="Provide the matching depot manifest in JSON format." label="info.json">
                        <Input accept=".json,application/json" id="info-file" onChange={(event) => setInfoFile(event.target.files?.[0] ?? null)} type="file" />
                      </FormField>

                      <div className="flex justify-end border-t pt-4">
                        <Button disabled={firmwareBusy !== null} onClick={() => void handleUploadDepotInfo()} type="button" variant="outline">
                          <Package className="size-4" />
                          Apply Manifest
                        </Button>
                      </div>
                    </CardContent>
                  </Card>
                </div>

                <Card className="overflow-hidden">
                  <CardHeader>
                    <CardTitle>Firmware Depots</CardTitle>
                    <CardDescription>Review depot snapshots and run release or rollback operations.</CardDescription>
                  </CardHeader>
                  <CardContent>
                    {dashboard.loading ? (
                      <div className="space-y-3">
                        {Array.from({ length: 6 }).map((_, index) => (
                          <Skeleton className="h-12 w-full" key={index} />
                        ))}
                      </div>
                    ) : dashboard.depots.length === 0 ? (
                      <EmptyState
                        description="Firmware depot snapshots will appear here when release data is available."
                        title="No firmware depots"
                      />
                    ) : (
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Depot</TableHead>
                            <TableHead>Stable</TableHead>
                            <TableHead>Beta</TableHead>
                            <TableHead>Testing</TableHead>
                            <TableHead>Rollback</TableHead>
                            <TableHead className="text-right">Files</TableHead>
                            <TableHead className="text-right">Actions</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {dashboard.depots.map((depot) => (
                            <TableRow key={depot.name}>
                              <TableCell className="font-medium">{depot.name}</TableCell>
                              <TableCell>{formatRelease(depot.stable)}</TableCell>
                              <TableCell>{formatRelease(depot.beta)}</TableCell>
                              <TableCell>{formatRelease(depot.testing)}</TableCell>
                              <TableCell>{formatRelease(depot.rollback)}</TableCell>
                              <TableCell className="text-right">{depot.info?.files?.length ?? 0}</TableCell>
                              <TableCell className="text-right">
                                <div className="space-y-2">
                                  <div className="flex justify-end gap-2">
                                    <Button
                                      disabled={firmwareBusy !== null || !canReleaseDepot(depot)}
                                      onClick={() => void handleDepotAction(depot.name, "release")}
                                      size="sm"
                                      type="button"
                                      variant="outline"
                                    >
                                      <Package className="size-4" />
                                      Release
                                    </Button>
                                    <Button
                                      disabled={firmwareBusy !== null || !canRollbackDepot(depot)}
                                      onClick={() => void handleDepotAction(depot.name, "rollback")}
                                      size="sm"
                                      type="button"
                                      variant="outline"
                                    >
                                      <RotateCcw className="size-4" />
                                      Rollback
                                    </Button>
                                  </div>
                                  <div className="text-xs text-muted-foreground">
                                    {depotActionHint(depot)}
                                  </div>
                                </div>
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    )}
                  </CardContent>
                </Card>
              </div>
            ) : null}

            {route.section === "memory" ? (
              <div className="grid gap-6 lg:grid-cols-2">
                <Card>
                  <CardHeader>
                    <CardTitle>Memory Module</CardTitle>
                    <CardDescription>This screen exists now so future operational areas have a real home in the sidebar.</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <EmptyState
                      description="The shell is ready for memory, cache, and store diagnostics without redesigning navigation again."
                      title="Coming soon"
                    />
                  </CardContent>
                </Card>
                <Card>
                  <CardHeader>
                    <CardTitle>Planned Surfaces</CardTitle>
                    <CardDescription>Examples of modules that can slot into this admin console next.</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <FutureModule
                      description="Active sessions, peer state, and cache inspection."
                      icon={MemoryStick}
                      title="In-memory state"
                    />
                    <FutureModule
                      description="Store backends, persistence health, and data debugging."
                      icon={Database}
                      title="KV stores"
                    />
                    <FutureModule
                      description="Background refreshes, OTA activity, and execution traces."
                      icon={Activity}
                      title="Jobs"
                    />
                  </CardContent>
                </Card>
              </div>
            ) : null}
          </div>
        </main>
      </div>
    </div>
  );
}

function SidebarButton({
  active,
  description,
  icon: Icon,
  label,
  onClick,
}: {
  active: boolean;
  description: string;
  icon: ComponentType<{ className?: string }>;
  label: string;
  onClick: () => void;
}): JSX.Element {
  return (
    <button
      className={cn(
        "flex w-full items-start gap-3 rounded-xl border px-3 py-3 text-left transition",
        active
          ? "border-slate-700 bg-slate-900 text-white"
          : "border-transparent text-slate-300 hover:border-slate-800 hover:bg-slate-900/70 hover:text-white",
      )}
      onClick={onClick}
      type="button"
    >
      <div className="mt-0.5 rounded-md bg-slate-800 p-2 text-slate-200">
        <Icon className="size-4" />
      </div>
      <div className="min-w-0 space-y-1">
        <div className="font-medium">{label}</div>
        <div className="text-sm leading-5 text-slate-400">{description}</div>
      </div>
    </button>
  );
}

function SidebarStat({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <div className="flex items-center justify-between rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
      <span className="text-slate-400">{label}</span>
      <span className="font-medium text-slate-100">{value}</span>
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
    <Card>
      <CardHeader className="space-y-3">
        <div className="flex items-center justify-between">
          <CardDescription>{label}</CardDescription>
          <div className="rounded-lg bg-primary/10 p-2 text-primary">
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

function ModuleCard({
  description,
  icon: Icon,
  title,
}: {
  description: string;
  icon: ComponentType<{ className?: string }>;
  title: string;
}): JSX.Element {
  return (
    <Card>
      <CardHeader className="space-y-3">
        <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Icon className="size-5" />
        </div>
        <div className="space-y-1">
          <CardTitle className="text-base">{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </div>
      </CardHeader>
    </Card>
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
    <Card>
      <CardHeader className="gap-3 pb-4">
        <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
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

function FutureModule({
  description,
  icon: Icon,
  title,
}: {
  description: string;
  icon: ComponentType<{ className?: string }>;
  title: string;
}): JSX.Element {
  return (
    <div className="flex items-start gap-3 rounded-lg border bg-background px-4 py-4">
      <div className="rounded-lg bg-primary/10 p-2 text-primary">
        <Icon className="size-4" />
      </div>
      <div className="space-y-1">
        <div className="font-medium">{title}</div>
        <div className="text-sm leading-6 text-muted-foreground">{description}</div>
      </div>
    </div>
  );
}

function FormField({
  children,
  description,
  label,
}: {
  children: ReactNode;
  description?: string;
  label: string;
}): JSX.Element {
  return (
    <div className="space-y-2 rounded-lg border bg-muted/20 p-4">
      <div className="space-y-1">
        <Label>{label}</Label>
        {description ? <p className="text-sm leading-6 text-muted-foreground">{description}</p> : null}
      </div>
      {children}
    </div>
  );
}

function DetailBlock({
  items,
  title,
}: {
  items: Array<[string, string | undefined]>;
  title: string;
}): JSX.Element {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {items.map(([label, value]) => (
          <div className="flex items-start justify-between gap-4 text-sm" key={label}>
            <span className="text-muted-foreground">{label}</span>
            <span className="max-w-[16rem] text-right text-foreground">{formatValue(value)}</span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

function StatusBadge({ status }: { status: string }): JSX.Element {
  if (status === "active") {
    return <Badge variant="success">Active</Badge>;
  }
  if (status === "blocked") {
    return <Badge variant="destructive">Blocked</Badge>;
  }
  return <Badge variant="outline">{status}</Badge>;
}

function ErrorBanner({ message }: { message: string }): JSX.Element {
  return (
    <div className="rounded-lg border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
      {message}
    </div>
  );
}

function NoticeBanner({ message, tone }: NoticeState): JSX.Element {
  if (tone === "error") {
    return <ErrorBanner message={message} />;
  }
  return (
    <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
      {message}
    </div>
  );
}

function EmptyState({
  description,
  title,
}: {
  description: string;
  title: string;
}): JSX.Element {
  return (
    <div className="flex min-h-56 flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border bg-muted/20 px-6 py-10 text-center">
      <div className="text-base font-medium">{title}</div>
      <p className="max-w-md text-sm leading-6 text-muted-foreground">{description}</p>
    </div>
  );
}

function useHashRoute(): [RouteState, (route: RouteState) => void] {
  const [route, setRoute] = useState<RouteState>(() => parseHash(window.location.hash));

  useEffect(() => {
    const handleHashChange = () => {
      setRoute(parseHash(window.location.hash));
    };

    window.addEventListener("hashchange", handleHashChange);
    return () => {
      window.removeEventListener("hashchange", handleHashChange);
    };
  }, []);

  const navigate = useCallback((next: RouteState) => {
    const hash = buildHash(next);
    if (window.location.hash === hash) {
      setRoute(next);
      return;
    }
    window.location.hash = hash;
  }, []);

  return [route, navigate];
}

function parseHash(hash: string): RouteState {
  const normalized = hash.replace(/^#/, "").trim();
  if (normalized === "") {
    return { section: "overview" };
  }

  const separator = normalized.indexOf("/");
  const sectionPart = separator === -1 ? normalized : normalized.slice(0, separator);
  const publicKey = separator === -1 ? undefined : normalized.slice(separator + 1);
  if (sectionPart === "devices") {
    return publicKey ? { publicKey: decodeURIComponent(publicKey), section: "devices" } : { section: "devices" };
  }
  if (sectionPart === "firmware" || sectionPart === "memory" || sectionPart === "overview") {
    return { section: sectionPart };
  }
  return { section: "overview" };
}

function buildHash(route: RouteState): string {
  if (route.section === "devices" && route.publicKey !== undefined) {
    return `#devices/${encodeURIComponent(route.publicKey)}`;
  }
  return `#${route.section}`;
}

function sectionMeta(route: RouteState): { description: string; eyebrow: string; title: string } {
  switch (route.section) {
    case "devices":
      return {
        description: route.publicKey
          ? "Device details now have their own screen structure with room for richer tabs and operational controls."
          : "Search devices from the list and open each one into a dedicated detail view.",
        eyebrow: route.publicKey ? `Devices / ${formatShortKey(route.publicKey)}` : "Devices",
        title: route.publicKey ? `Device ${formatShortKey(route.publicKey)}` : "Devices",
      };
    case "firmware":
      return {
        description: "A table-oriented admin section for firmware rollout visibility and future release actions.",
        eyebrow: "Firmware",
        title: "Firmware",
      };
    case "memory":
      return {
        description: "A reserved section in the sidebar so more admin modules can grow without redesigning the shell.",
        eyebrow: "Memory",
        title: "Memory",
      };
    case "overview":
    default:
      return {
        description: "A central overview for server health, devices, and firmware activity.",
        eyebrow: "Overview",
        title: "Overview",
      };
  }
}

function formatServerTime(value: number | undefined): string {
  if (value === undefined) {
    return "Server time unavailable";
  }
  return new Date(value).toLocaleString();
}

function formatRelease(release: DepotRelease | undefined): string {
  return release?.firmware_semver && release.firmware_semver !== "" ? release.firmware_semver : "—";
}

function hasRelease(release: DepotRelease | undefined): boolean {
  return Boolean(release?.firmware_semver && release.firmware_semver !== "");
}

function canReleaseDepot(depot: Depot): boolean {
  return hasRelease(depot.beta) && hasRelease(depot.testing);
}

function canRollbackDepot(depot: Depot): boolean {
  return hasRelease(depot.rollback);
}

function depotActionHint(depot: Depot): string {
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

function formatShortKey(value: string | undefined): string {
  if (value === undefined || value === "") {
    return "No public key";
  }
  if (value.length <= 18) {
    return value;
  }
  return `${value.slice(0, 10)}...${value.slice(-6)}`;
}

function formatValue(value: string | undefined): string {
  if (value === undefined || value === "") {
    return "—";
  }
  return isDateTimeLike(value) ? formatDate(value) : value;
}

function formatDate(value: string | undefined): string {
  if (value === undefined || value === "") {
    return "—";
  }
  const time = Date.parse(value);
  if (Number.isNaN(time)) {
    return value;
  }
  return new Date(time).toLocaleString();
}

function isDateTimeLike(value: string): boolean {
  return value.includes("T") || value.endsWith("Z");
}

function deviceTitle(info: DeviceInfo | null | undefined, publicKey: string): string {
  return info?.name?.trim() ? info.name : formatShortKey(publicKey);
}

const root = document.querySelector<HTMLElement>("#app");

if (root === null) {
  throw new Error("missing #app root");
}

createRoot(root).render(<App />);
