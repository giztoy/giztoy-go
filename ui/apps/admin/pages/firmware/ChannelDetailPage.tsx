import { useCallback, useMemo, useState } from "react";
import { ChevronLeft, RefreshCw, Upload } from "lucide-react";
import { Link, useParams } from "react-router-dom";

import { expectData, toMessage } from "../../../../packages/components/api";
import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../../packages/components/card";
import { Input } from "../../../../packages/components/input";
import { Skeleton } from "../../../../packages/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../../packages/components/table";

import { putChannel } from "../../../../packages/adminservice";

import { ErrorBanner, NoticeBanner } from "../../components/banners";
import { EmptyState } from "../../components/empty-state";
import { FormField } from "../../components/form-field";
import { PageBreadcrumb } from "../../components/page-breadcrumb";
import { useChannelDetail } from "../../hooks/useChannelDetail";
import { formatRelease } from "../../lib/format";

const CHANNEL_OPTIONS = ["stable", "beta", "testing"] as const;

export function ChannelDetailPage(): JSX.Element {
  const params = useParams();
  const rawDepot = params.depot ?? "";
  const rawChannel = params.channel ?? "";

  const depotName = useMemo(() => {
    try {
      return decodeURIComponent(rawDepot);
    } catch {
      return rawDepot;
    }
  }, [rawDepot]);

  const channelName = useMemo(() => {
    try {
      return decodeURIComponent(rawChannel);
    } catch {
      return rawChannel;
    }
  }, [rawChannel]);

  const isSupportedChannel = CHANNEL_OPTIONS.includes(channelName as (typeof CHANNEL_OPTIONS)[number]);

  const detail = useChannelDetail(
    depotName === "" ? undefined : depotName,
    channelName === "" || !isSupportedChannel ? undefined : channelName,
  );
  const depotPath = encodeURIComponent(depotName);

  const [notice, setNotice] = useState<{ message: string; tone: "error" | "success" } | null>(null);
  const [uploadBusy, setUploadBusy] = useState(false);
  const [uploadFile, setUploadFile] = useState<File | null>(null);

  const handleUpload = useCallback(async () => {
    if (uploadFile === null) {
      setNotice({ message: "Select a firmware tarball first.", tone: "error" });
      return;
    }
    setUploadBusy(true);
    setNotice(null);
    try {
      await expectData(
        putChannel({
          body: uploadFile,
          path: { channel: channelName, depot: depotName },
        }),
      );
      setNotice({
        message: `Uploaded ${uploadFile.name} to ${depotName}/${channelName}.`,
        tone: "success",
      });
      await detail.reload();
    } catch (error) {
      setNotice({ message: toMessage(error), tone: "error" });
    } finally {
      setUploadBusy(false);
    }
  }, [channelName, depotName, detail, uploadFile]);

  if (depotName === "" || channelName === "") {
    return <EmptyState description="Missing depot or channel in the URL." title="Invalid route" />;
  }

  if (!isSupportedChannel) {
    return <EmptyState description="Supported channels are stable, beta, and testing. Rollback is an action, not a channel." title="Invalid channel" />;
  }

  return (
    <div className="space-y-6">
      <PageBreadcrumb
        items={[
          { href: "/overview", label: "Overview" },
          { href: "/firmware", label: "Firmware" },
          { href: `/firmware/${depotPath}`, label: depotName },
          { label: channelName },
        ]}
      />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Firmware · Channel</div>
          <h1 className="text-3xl font-semibold tracking-tight capitalize">
            {depotName} / {channelName}
          </h1>
          <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">
            Inspect the current channel release and upload a new tarball to this lane.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button asChild size="sm" variant="outline">
            <Link to={`/firmware/${depotPath}`}>
              <ChevronLeft className="size-4" />
              Back to depot
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

      <Card>
        <CardHeader>
          <CardTitle>Upload release</CardTitle>
          <CardDescription>Upload a release archive into this depot channel.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <FormField description="Upload the release archive produced by your firmware build." label="Release tarball">
            <Input
              accept=".tar,.tgz,.tar.gz,application/octet-stream"
              id="upload-file"
              onChange={(event) => setUploadFile(event.target.files?.[0] ?? null)}
              type="file"
            />
          </FormField>
          <div className="flex justify-end border-t pt-4">
            <Button disabled={uploadBusy} onClick={() => void handleUpload()} type="button">
              <Upload className="size-4" />
              Upload release
            </Button>
          </div>
        </CardContent>
      </Card>

      {detail.loading ? (
        <div className="space-y-4">
          <Skeleton className="h-32 w-full" />
        </div>
      ) : detail.error !== "" ? (
        <ErrorBanner message={detail.error} />
      ) : detail.data === null ? (
        <EmptyState description="This channel could not be loaded." title="Not found" />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Current release</CardTitle>
            <CardDescription>Version: {formatRelease(detail.data)}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Path</TableHead>
                    <TableHead>SHA256</TableHead>
                    <TableHead>MD5</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {detail.data.files && detail.data.files.length > 0 ? (
                    detail.data.files.map((file) => (
                      <TableRow key={`${file.path}-${file.sha256}`}>
                        <TableCell className="font-mono text-xs">{file.path}</TableCell>
                        <TableCell className="max-w-[12rem] truncate font-mono text-xs">{file.sha256}</TableCell>
                        <TableCell className="max-w-[10rem] truncate font-mono text-xs">{file.md5}</TableCell>
                      </TableRow>
                    ))
                  ) : (
                    <TableRow>
                      <TableCell className="text-muted-foreground" colSpan={3}>
                        No files listed for this release.
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>
            <div>
              <div className="mb-2 text-sm font-medium">Raw payload</div>
              <pre className="overflow-x-auto rounded-lg border bg-muted/50 p-4 text-xs leading-6 text-foreground">
                {JSON.stringify(detail.data, null, 2)}
              </pre>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
