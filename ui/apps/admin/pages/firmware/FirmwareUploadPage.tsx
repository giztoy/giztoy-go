import { useCallback, useState } from "react";
import { ChevronLeft, Package, Upload } from "lucide-react";
import { Link } from "react-router-dom";

import { expectData, toMessage } from "../../../../packages/components/api";
import { putChannel, putDepotInfo, type DepotInfo as AdminDepotInfo } from "../../../../packages/adminservice";
import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../../packages/components/card";
import { Input } from "../../../../packages/components/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../../../packages/components/select";

import { ErrorBanner, NoticeBanner } from "../../../../packages/components/banners";
import { FormField } from "../../../../packages/components/form-field";
import { PageBreadcrumb } from "../../../../packages/components/page-breadcrumb";

const CHANNEL_OPTIONS = ["stable", "beta", "testing"] as const;

export function FirmwareUploadPage(): JSX.Element {
  const [notice, setNotice] = useState<{ message: string; tone: "error" | "success" } | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [uploadDepot, setUploadDepot] = useState("");
  const [uploadChannel, setUploadChannel] = useState<(typeof CHANNEL_OPTIONS)[number]>("stable");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [infoDepot, setInfoDepot] = useState("");
  const [infoFile, setInfoFile] = useState<File | null>(null);

  const handleUploadFirmware = useCallback(async () => {
    if (uploadDepot.trim() === "" || uploadFile === null) {
      setNotice({ message: "Select a depot and a firmware tarball first.", tone: "error" });
      return;
    }
    setBusy("upload");
    setNotice(null);
    try {
      await expectData(
        putChannel({
          body: uploadFile,
          path: { channel: uploadChannel, depot: uploadDepot.trim() },
        }),
      );
      setNotice({
        message: `Uploaded ${uploadFile.name} to ${uploadDepot.trim()}/${uploadChannel}.`,
        tone: "success",
      });
    } catch (uploadError) {
      setNotice({ message: toMessage(uploadError), tone: "error" });
    } finally {
      setBusy(null);
    }
  }, [uploadChannel, uploadDepot, uploadFile]);

  const handleUploadDepotInfo = useCallback(async () => {
    if (infoDepot.trim() === "" || infoFile === null) {
      setNotice({ message: "Select a depot and an info.json file first.", tone: "error" });
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
          path: { depot: infoDepot.trim() },
        }),
      );
      setNotice({
        message: `Updated metadata for depot ${infoDepot.trim()}.`,
        tone: "success",
      });
    } catch (infoError) {
      setNotice({ message: toMessage(infoError), tone: "error" });
    } finally {
      setBusy(null);
    }
  }, [infoDepot, infoFile]);

  const errorNotice = notice?.tone === "error" ? notice.message : "";
  const successNotice = notice?.tone === "success" ? notice.message : "";

  return (
    <div className="space-y-6">
      <PageBreadcrumb
        items={[
          { href: "/overview", label: "Overview" },
          { href: "/firmware", label: "Firmware" },
          { label: "Add firmware" },
        ]}
      />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Firmware</div>
          <h1 className="text-3xl font-semibold tracking-tight">Add firmware</h1>
          <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">
            Publish a release tarball or update a depot manifest without mixing those forms into the depot list.
          </p>
        </div>
        <Button asChild className="h-8 min-w-fit shrink-0 whitespace-nowrap px-3 text-sm" variant="outline">
          <Link to="/firmware">
            <span className="inline-flex items-center gap-2 whitespace-nowrap">
              <ChevronLeft className="size-4" />
              Back to depots
            </span>
          </Link>
        </Button>
      </div>

      {errorNotice !== "" ? <ErrorBanner message={errorNotice} /> : null}
      {successNotice !== "" ? <NoticeBanner message={successNotice} tone="success" /> : null}

      <div className="grid gap-6 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Upload firmware</CardTitle>
            <CardDescription>Upload a release tarball into a depot channel, even before the depot appears in the list.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <FormField description="The firmware depot receiving this release." label="Depot">
                <Input onChange={(event) => setUploadDepot(event.target.value)} placeholder="demo-main" value={uploadDepot} />
              </FormField>
              <FormField description="Choose which rollout lane this tarball should land in." label="Channel">
                <Select onValueChange={(value) => setUploadChannel(value as (typeof CHANNEL_OPTIONS)[number])} value={uploadChannel}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select channel" />
                  </SelectTrigger>
                  <SelectContent>
                    {CHANNEL_OPTIONS.map((channel) => (
                      <SelectItem key={channel} value={channel}>
                        {channel}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>
            </div>
            <FormField description="Upload the release archive produced by your firmware build." label="Release tarball">
              <Input
                accept=".tar,.tgz,.tar.gz,application/octet-stream"
                onChange={(event) => setUploadFile(event.target.files?.[0] ?? null)}
                type="file"
              />
            </FormField>
            <div className="flex justify-end border-t pt-4">
              <Button disabled={busy !== null} onClick={() => void handleUploadFirmware()} type="button">
                <Upload className="size-4" />
                Upload Release
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Update depot info</CardTitle>
            <CardDescription>Upload an `info.json` manifest for a depot before drilling into the detail page.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <FormField description="The target depot whose file manifest should be updated." label="Depot">
              <Input onChange={(event) => setInfoDepot(event.target.value)} placeholder="demo-main" value={infoDepot} />
            </FormField>
            <FormField description="Provide the matching depot manifest in JSON format." label="info.json">
              <Input accept=".json,application/json" onChange={(event) => setInfoFile(event.target.files?.[0] ?? null)} type="file" />
            </FormField>
            <div className="flex justify-end border-t pt-4">
              <Button disabled={busy !== null} onClick={() => void handleUploadDepotInfo()} type="button" variant="outline">
                <Package className="size-4" />
                Apply Manifest
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
