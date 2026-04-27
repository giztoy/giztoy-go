import { useCallback, useMemo, useState } from "react";
import { ChevronLeft, ChevronRight, Package, RefreshCw, RotateCcw } from "lucide-react";
import { Link, useParams } from "react-router-dom";

import { expectData, toMessage } from "../../../../packages/components/api";
import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../../packages/components/card";
import { Input } from "../../../../packages/components/input";
import { Skeleton } from "../../../../packages/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../../packages/components/table";

import { putDepotInfo, releaseDepot, rollbackDepot, type DepotInfo as AdminDepotInfo } from "../../../../packages/adminservice";

import { ErrorBanner, NoticeBanner } from "../../../../packages/components/banners";
import { DetailBlock } from "../../../../packages/components/detail-block";
import { EmptyState } from "../../../../packages/components/empty-state";
import { FormField } from "../../../../packages/components/form-field";
import { PageBreadcrumb } from "../../../../packages/components/page-breadcrumb";
import { useDepotDetail } from "../../hooks/useDepotDetail";
import { canReleaseDepot, canRollbackDepot, depotActionHint } from "../../lib/firmware-helpers";
import { formatRelease } from "../../lib/format";

const CHANNELS = ["stable", "beta", "testing"] as const;

export function DepotDetailPage(): JSX.Element {
  const params = useParams();
  const rawDepot = params.depot ?? "";
  const depotName = useMemo(() => {
    try {
      return decodeURIComponent(rawDepot);
    } catch {
      return rawDepot;
    }
  }, [rawDepot]);

  const detail = useDepotDetail(depotName === "" ? undefined : depotName);
  const depotPath = encodeURIComponent(depotName);

  const [notice, setNotice] = useState<{ message: string; tone: "error" | "success" } | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [infoFile, setInfoFile] = useState<File | null>(null);

  const runAction = useCallback(async (name: string, action: () => Promise<void>, successMessage: string) => {
    setBusy(name);
    setNotice(null);
    try {
      await action();
      setNotice({ message: successMessage, tone: "success" });
      await detail.reload();
    } catch (error) {
      setNotice({ message: toMessage(error), tone: "error" });
    } finally {
      setBusy(null);
    }
  }, [detail]);

  const handleRelease = useCallback(async () => {
    await runAction("release", async () => {
      await expectData(releaseDepot({ path: { depot: depotName } }));
    }, `Released depot ${depotName}.`);
  }, [depotName, runAction]);

  const handleRollback = useCallback(async () => {
    await runAction("rollback", async () => {
      await expectData(rollbackDepot({ path: { depot: depotName } }));
    }, `Rolled back depot ${depotName}.`);
  }, [depotName, runAction]);

  const handleApplyManifest = useCallback(async () => {
    if (infoFile === null) {
      setNotice({ message: "Select an info.json file first.", tone: "error" });
      return;
    }
    setBusy("info");
    setNotice(null);
    try {
      const text = await infoFile.text();
      const parsed = JSON.parse(text) as AdminDepotInfo;
      await expectData(
        putDepotInfo({
          body: parsed,
          path: { depot: depotName },
        }),
      );
      setNotice({ message: `Updated metadata for depot ${depotName}.`, tone: "success" });
      await detail.reload();
    } catch (error) {
      setNotice({ message: toMessage(error), tone: "error" });
    } finally {
      setBusy(null);
    }
  }, [depotName, detail, infoFile]);

  if (depotName === "") {
    return <EmptyState description="Missing depot name in the URL." title="Invalid route" />;
  }

  const depot = detail.data;

  return (
    <div className="space-y-6">
      <PageBreadcrumb
        items={[
          { href: "/overview", label: "Overview" },
          { href: "/firmware", label: "Firmware" },
          { label: depotName },
        ]}
      />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Firmware · Depot</div>
          <h1 className="text-3xl font-semibold tracking-tight">{depotName}</h1>
          <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">
            Release testing to stable, roll back as a separate action, and open individual channels for uploads.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button asChild size="sm" variant="outline">
            <Link to="/firmware">
              <ChevronLeft className="size-4" />
              Back to depots
            </Link>
          </Button>
          <Button className="min-w-fit shrink-0 whitespace-nowrap" onClick={() => void detail.reload()} size="sm" variant="outline">
            <span className="inline-flex items-center gap-2 whitespace-nowrap">
              <RefreshCw className="size-4" />
              Reload
            </span>
          </Button>
        </div>
      </div>

      {notice !== null ? <NoticeBanner message={notice.message} tone={notice.tone} /> : null}

      {detail.loading ? (
        <div className="space-y-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-48 w-full" />
        </div>
      ) : detail.error !== "" ? (
        <ErrorBanner message={detail.error} />
      ) : depot === null ? (
        <EmptyState description="This depot could not be loaded." title="Not found" />
      ) : (
        <div className="space-y-6">
          <div className="grid gap-4 lg:grid-cols-[1fr_1fr]">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Depot operations</CardTitle>
                <CardDescription>Promote testing to stable or restore the rollback snapshot as a separate action.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-sm text-muted-foreground">{depotActionHint(depot)}</p>
                <div className="flex flex-wrap gap-2">
                  <Button disabled={busy !== null || !canReleaseDepot(depot)} onClick={() => void handleRelease()} type="button" variant="outline">
                    <Package className="size-4" />
                    Release
                  </Button>
                  <Button disabled={busy !== null || !canRollbackDepot(depot)} onClick={() => void handleRollback()} type="button" variant="outline">
                    <RotateCcw className="size-4" />
                    Rollback
                  </Button>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Update depot info</CardTitle>
                <CardDescription>Upload an `info.json` manifest for this depot.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <FormField description="Provide the matching depot manifest in JSON format." label="info.json">
                  <Input accept=".json,application/json" id="info-file" onChange={(event) => setInfoFile(event.target.files?.[0] ?? null)} type="file" />
                </FormField>
                <div className="flex justify-end border-t pt-4">
                  <Button disabled={busy !== null} onClick={() => void handleApplyManifest()} type="button" variant="outline">
                    <Package className="size-4" />
                    Apply manifest
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <DetailBlock
              items={[
                ["Depot", depot.name],
                ["Manifest files", String(depot.info?.files?.length ?? 0)],
                ["Stable", formatRelease(depot.stable)],
                ["Beta", formatRelease(depot.beta)],
              ]}
              title="Snapshot"
            />
            <DetailBlock
              items={[
                ["Testing", formatRelease(depot.testing)],
                ["Rollback", formatRelease(depot.rollback)],
              ]}
              title="Testing And Rollback Snapshot"
            />
          </div>

          <Card>
            <CardHeader>
              <CardTitle>Channels</CardTitle>
              <CardDescription>Open one of the three release channels to inspect files and upload a new release tarball.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Channel</TableHead>
                      <TableHead>Version</TableHead>
                      <TableHead className="text-right">Files</TableHead>
                      <TableHead className="text-right">Open</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {CHANNELS.map((channel) => {
                      const release = depot[channel];
                      return (
                        <TableRow className="hover:bg-muted/40" key={channel}>
                          <TableCell className="font-medium capitalize">{channel}</TableCell>
                          <TableCell>{formatRelease(release)}</TableCell>
                          <TableCell className="text-right">{release?.files?.length ?? 0}</TableCell>
                          <TableCell className="text-right text-muted-foreground">
                            <Link to={`/firmware/${depotPath}/${encodeURIComponent(channel)}`}>
                              <ChevronRight className="ml-auto size-4" />
                            </Link>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
