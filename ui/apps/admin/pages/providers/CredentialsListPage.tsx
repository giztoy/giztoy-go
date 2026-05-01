import { useMemo, useState } from "react";
import { Eye, EyeOff, RefreshCw, X } from "lucide-react";

import { Badge } from "../../../../packages/components/badge";
import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../../packages/components/card";
import { Skeleton } from "../../../../packages/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../../packages/components/table";
import { expectData } from "../../../../packages/components/api";
import { listCredentials, type Credential } from "../../../../packages/adminservice";

import { ErrorBanner } from "../../../../packages/components/banners";
import { EmptyState } from "../../../../packages/components/empty-state";
import { PageBreadcrumb } from "../../../../packages/components/page-breadcrumb";
import { useCursorListPage } from "../../hooks/useCursorListPage";
import { formatDate } from "../../lib/format";

export function CredentialsListPage(): JSX.Element {
  const [selectedCredential, setSelectedCredential] = useState<Credential | null>(null);
  const { error, hasNext, items, loading, nextPage, pageNumber, prevPage, refresh } = useCursorListPage<Credential>(async (query) => {
    const result = await expectData(listCredentials({ query }));
    return {
      hasNext: result.has_next,
      items: result.items ?? [],
      nextCursor: result.next_cursor ?? null,
    };
  });

  return (
    <div className="space-y-6">
      <PageBreadcrumb items={[{ href: "/overview", label: "Overview" }, { label: "Credentials" }]} />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Providers</div>
          <h1 className="text-3xl font-semibold tracking-tight">Credentials</h1>
          <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">
            Shared provider credentials used by services like MiniMax tenants and future external integrations.
          </p>
        </div>
        <Button className="h-8 min-w-fit shrink-0 whitespace-nowrap px-3 text-sm" onClick={() => void refresh()} variant="outline">
          <span className="inline-flex items-center gap-2 whitespace-nowrap">
            <RefreshCw className="size-4" />
            Refresh
          </span>
        </Button>
      </div>

      {error !== "" ? <ErrorBanner message={error} /> : null}

      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
          <div className="space-y-1">
            <CardTitle>Credential catalog</CardTitle>
            <CardDescription>Stored authentication entries keyed by provider and method.</CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <Badge variant="outline">Page {pageNumber}</Badge>
            <Badge variant="secondary">{items.length} loaded</Badge>
            {hasNext ? <Badge variant="outline">More Available</Badge> : null}
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex justify-end gap-2">
            <Button
              className="h-8 min-w-fit shrink-0 whitespace-nowrap px-3 text-sm disabled:border-border disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100 disabled:shadow-none"
              disabled={loading || pageNumber === 1}
              onClick={prevPage}
              type="button"
              variant="outline"
            >
              Previous
            </Button>
            <Button
              className="h-8 min-w-fit shrink-0 whitespace-nowrap px-3 text-sm disabled:border-border disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100 disabled:shadow-none"
              disabled={loading || !hasNext}
              onClick={nextPage}
              type="button"
              variant="outline"
            >
              Next
            </Button>
          </div>

          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 6 }).map((_, index) => (
                <Skeleton className="h-14 w-full" key={index} />
              ))}
            </div>
          ) : items.length === 0 ? (
            <EmptyState description="Credentials will appear here after they are created through the admin API." title="No credentials" />
          ) : (
            <div className="rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Provider</TableHead>
                    <TableHead>Method</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead className="text-right">Body Keys</TableHead>
                    <TableHead>Updated</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((credential) => (
                    <TableRow key={credential.name}>
                      <TableCell className="font-medium">{credential.name}</TableCell>
                      <TableCell>{credential.provider}</TableCell>
                      <TableCell>
                        <Badge variant="outline">{credential.method}</Badge>
                      </TableCell>
                      <TableCell className="max-w-[22rem] text-sm text-muted-foreground">{credential.description?.trim() || "—"}</TableCell>
                      <TableCell className="text-right">
                        <Button
                          aria-label={`View body keys for ${credential.name}`}
                          className="h-8 min-w-fit gap-2 px-2 text-xs"
                          onClick={() => setSelectedCredential(credential)}
                          type="button"
                          variant="outline"
                        >
                          <Eye className="size-3.5" />
                          {Object.keys(credential.body).length}
                        </Button>
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">{formatDate(credential.updated_at)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
      {selectedCredential != null ? <CredentialBodyDialog credential={selectedCredential} onClose={() => setSelectedCredential(null)} /> : null}
    </div>
  );
}

function CredentialBodyDialog({ credential, onClose }: { credential: Credential; onClose: () => void }): JSX.Element {
  const [revealed, setRevealed] = useState(false);
  const entries = useMemo(() => Object.entries(credential.body), [credential.body]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4" role="presentation">
      <div
        aria-modal="true"
        className="w-full max-w-3xl rounded-xl border bg-background shadow-xl"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
      >
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div className="space-y-1">
            <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Credential body</div>
            <h2 className="text-lg font-semibold">{credential.name}</h2>
            <p className="text-sm text-muted-foreground">
              {credential.provider} · {credential.method}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button className="h-8 gap-2 px-3 text-xs" onClick={() => setRevealed((value) => !value)} type="button" variant="outline">
              {revealed ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
              {revealed ? "Hide values" : "Show values"}
            </Button>
            <Button aria-label="Close credential body dialog" className="h-8 w-8 p-0" onClick={onClose} type="button" variant="ghost">
              <X className="size-4" />
            </Button>
          </div>
        </div>
        <div className="max-h-[60vh] overflow-auto p-5">
          {entries.length === 0 ? (
            <EmptyState description="This credential has an empty body." title="No body keys" />
          ) : (
            <div className="rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-56">Key</TableHead>
                    <TableHead>Value</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {entries.map(([key, value]) => {
                    const formatted = formatCredentialBodyValue(value);
                    return (
                      <TableRow key={key}>
                        <TableCell className="font-mono text-xs font-medium">{key}</TableCell>
                        <TableCell className="break-all font-mono text-xs">{revealed ? formatted : maskCredentialBodyValue(formatted)}</TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function formatCredentialBodyValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (value == null) {
    return "";
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function maskCredentialBodyValue(value: string): string {
  if (value === "") {
    return "—";
  }
  if (value.length <= 2) {
    return "**";
  }
  if (value.length <= 8) {
    return `${value.slice(0, 1)}****${value.slice(-1)}`;
  }
  return `${value.slice(0, 6)}******${value.slice(-4)}`;
}
