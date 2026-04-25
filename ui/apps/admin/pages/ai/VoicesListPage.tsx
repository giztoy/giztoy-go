import { RefreshCw } from "lucide-react";

import { Badge } from "../../../../packages/components/badge";
import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../../packages/components/card";
import { Skeleton } from "../../../../packages/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../../packages/components/table";
import { expectData } from "../../../../packages/components/api";
import { listVoices, type Voice } from "../../../../packages/adminservice";

import { ErrorBanner } from "../../components/banners";
import { EmptyState } from "../../components/empty-state";
import { PageBreadcrumb } from "../../components/page-breadcrumb";
import { useCursorListPage } from "../../hooks/useCursorListPage";
import { formatDate, formatValue } from "../../lib/format";

export function VoicesListPage(): JSX.Element {
  const { error, hasNext, items, loading, nextPage, pageNumber, prevPage, refresh } = useCursorListPage<Voice>(async (query) => {
    const result = await expectData(listVoices({ query }));
    return {
      hasNext: result.has_next,
      items: result.items ?? [],
      nextCursor: result.next_cursor ?? null,
    };
  });

  return (
    <div className="space-y-6">
      <PageBreadcrumb items={[{ href: "/overview", label: "Overview" }, { label: "Voices" }]} />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">AI</div>
          <h1 className="text-3xl font-semibold tracking-tight">Voices</h1>
          <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">
            Global voice catalog across providers, including both manually managed entries and synced upstream voices.
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
            <CardTitle>Voice catalog</CardTitle>
            <CardDescription>Provider voices stored in the shared catalog and ready for downstream use.</CardDescription>
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
            <EmptyState description="Voices will appear here after manual creation or provider sync." title="No voices" />
          ) : (
            <div className="rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Name</TableHead>
                    <TableHead>Source</TableHead>
                    <TableHead>Provider</TableHead>
                    <TableHead>Provider Voice ID</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Last Sync</TableHead>
                    <TableHead>Updated</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((voice) => (
                    <TableRow key={voice.id}>
                      <TableCell className="font-medium">{voice.id}</TableCell>
                      <TableCell>{voice.name?.trim() || "—"}</TableCell>
                      <TableCell>
                        <Badge variant={voice.source === "sync" ? "secondary" : "outline"}>{voice.source}</Badge>
                      </TableCell>
                      <TableCell className="text-sm">
                        <div>{voice.provider.kind}</div>
                        <div className="text-muted-foreground">{voice.provider.name}</div>
                      </TableCell>
                      <TableCell className="font-mono text-xs">{formatValue(voice.provider_voice_id)}</TableCell>
                      <TableCell>{formatValue(voice.provider_voice_type)}</TableCell>
                      <TableCell className="text-sm text-muted-foreground">{formatDate(voice.synced_at)}</TableCell>
                      <TableCell className="text-sm text-muted-foreground">{formatDate(voice.updated_at)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
