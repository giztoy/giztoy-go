import type { JSX } from "react";
import { useCallback, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { Cpu, Info, RadioTower, Settings2 } from "lucide-react";

import { expectData, toMessage } from "../../packages/components/api";
import { Badge } from "../../packages/components/badge";
import { NoticeBanner } from "../../packages/components/banners";
import { Button } from "../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../packages/components/card";
import { EmptyState } from "../../packages/components/empty-state";
import { Skeleton } from "../../packages/components/skeleton";
import { cn } from "../../packages/components/utils";
import { getConfig, getInfo, getOta } from "../../packages/gearservice";
import { client as gearClient } from "../../packages/gearservice/client.gen";
import { getServerInfo } from "../../packages/serverpublic";
import { client as publicClient } from "../../packages/serverpublic/client.gen";

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

interface ActionDefinition {
  description: string;
  id: string;
  label: string;
  run: () => Promise<unknown>;
}

interface NoticeState {
  message: string;
  tone: "error" | "success";
}

const actions: ActionDefinition[] = [
  {
    description: "Confirm the current server identity and build metadata exposed to the device.",
    id: "server-info",
    label: "Server Info",
    run: () => expectData(getServerInfo()),
  },
  {
    description: "Read the current device information returned by the proxied public endpoint.",
    id: "device-info",
    label: "Device Info",
    run: () => expectData(getInfo()),
  },
  {
    description: "Inspect the effective device configuration visible from the play flow.",
    id: "configuration",
    label: "Configuration",
    run: () => expectData(getConfig()),
  },
  {
    description: "Preview the OTA summary that the device would use for firmware decisions.",
    id: "ota-summary",
    label: "OTA Summary",
    run: () => expectData(getOta()),
  },
];

function App(): JSX.Element {
  const [activeAction, setActiveAction] = useState<string | null>(null);
  const [notice, setNotice] = useState<NoticeState | null>(null);
  const [output, setOutput] = useState<string>("Ready.");

  const activeLabel = useMemo(
    () => actions.find((action) => action.id === activeAction)?.label ?? null,
    [activeAction],
  );

  const runAction = useCallback(async (action: ActionDefinition) => {
    setActiveAction(action.id);
    setNotice(null);
    try {
      const data = await action.run();
      setOutput(JSON.stringify(data, null, 2));
      setNotice({ message: `${action.label} loaded successfully.`, tone: "success" });
    } catch (error) {
      setOutput("");
      setNotice({ message: toMessage(error), tone: "error" });
    } finally {
      setActiveAction(null);
    }
  }, []);

  return (
    <main className="min-h-screen bg-background">
      <div className="mx-auto flex min-h-screen w-full max-w-6xl flex-col gap-8 px-4 py-8 lg:px-8 lg:py-12">
        <section className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
          <Card className="border-border/70 shadow-sm">
            <CardHeader className="gap-4">
              <div className="flex flex-wrap items-center gap-3">
                <Badge>Play</Badge>
                <Badge variant="secondary">Proxy API bases: /api/gear and /api/public</Badge>
              </div>
              <div className="space-y-2">
                <CardTitle className="text-3xl font-semibold tracking-tight">GizClaw Play</CardTitle>
                <CardDescription className="max-w-2xl text-base leading-7">
                  A lightweight operator surface for probing the current gear endpoints and server info through the local proxy.
                </CardDescription>
              </div>
            </CardHeader>
          </Card>

          <Card className="border-border/70 bg-muted/20 shadow-none">
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Current State</CardTitle>
              <CardDescription>Quick feedback while you inspect the proxied responses.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <InfoRow label="Status" value={activeLabel ? `Loading ${activeLabel}` : "Idle"} />
              <InfoRow label="Surface" value="Gear and server endpoints" />
              <InfoRow label="Transport" value="Local reverse proxy" />
            </CardContent>
          </Card>
        </section>

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {actions.map((action) => (
            <Card className="border-border/70" key={action.id}>
              <CardHeader className="space-y-3">
                <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <ActionIcon id={action.id} />
                </div>
                <div className="space-y-1">
                  <CardTitle className="text-base">{action.label}</CardTitle>
                  <CardDescription className="leading-6">{action.description}</CardDescription>
                </div>
              </CardHeader>
              <CardContent>
                <Button
                  className="w-full"
                  disabled={activeAction !== null}
                  onClick={() => void runAction(action)}
                  type="button"
                  variant={activeAction === action.id ? "secondary" : "default"}
                >
                  {activeAction === action.id ? "Loading..." : `Run ${action.label}`}
                </Button>
              </CardContent>
            </Card>
          ))}
        </section>

        <Card className="min-h-[26rem] border-border/70">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Response</CardTitle>
            <CardDescription>Most recent payload returned by the selected proxied endpoint.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {notice ? <NoticeBanner message={notice.message} tone={notice.tone} /> : null}

            {activeAction !== null ? (
              <div className="space-y-3">
                <Skeleton className="h-5 w-40" />
                <Skeleton className="h-56 w-full" />
              </div>
            ) : output === "" ? (
              <EmptyState
                description="Run one of the actions above to load a response from the proxied API."
                title="No response available"
              />
            ) : (
              <pre
                className={cn(
                  "overflow-x-auto rounded-xl border border-slate-800 bg-slate-950 p-4 text-sm leading-6 text-slate-100",
                  "min-h-[20rem] whitespace-pre-wrap break-words",
                )}
              >
                {output}
              </pre>
            )}
          </CardContent>
        </Card>
      </div>
    </main>
  );
}

function InfoRow({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <div className="flex items-start justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium text-foreground">{value}</span>
    </div>
  );
}

function ActionIcon({ id }: { id: string }): JSX.Element {
  switch (id) {
    case "server-info":
      return <RadioTower className="size-4" />;
    case "device-info":
      return <Info className="size-4" />;
    case "configuration":
      return <Settings2 className="size-4" />;
    case "ota-summary":
      return <Cpu className="size-4" />;
    default:
      return <Info className="size-4" />;
  }
}

const root = document.querySelector<HTMLElement>("#app");

if (root === null) {
  throw new Error("missing #app root");
}

createRoot(root).render(<App />);
