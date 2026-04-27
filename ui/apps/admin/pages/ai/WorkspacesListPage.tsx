import { RefreshCw } from "lucide-react";

import { Badge } from "../../../../packages/components/badge";
import { Button } from "../../../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../../packages/components/card";
import { Skeleton } from "../../../../packages/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../../../packages/components/table";
import { expectData } from "../../../../packages/components/api";
import { listWorkspaces, type Workspace } from "../../../../packages/adminservice";

import { ErrorBanner } from "../../../../packages/components/banners";
import { EmptyState } from "../../../../packages/components/empty-state";
import { PageBreadcrumb } from "../../../../packages/components/page-breadcrumb";
import { useCursorListPage } from "../../hooks/useCursorListPage";
import { formatDate } from "../../lib/format";

export function WorkspacesListPage(): JSX.Element {
  const { error, hasNext, items, loading, nextPage, pageNumber, prevPage, refresh } = useCursorListPage<Workspace>(async (query) => {
    const result = await expectData(listWorkspaces({ query }));
    return {
      hasNext: result.has_next,
      items: result.items ?? [],
      nextCursor: result.next_cursor ?? null,
    };
  });

  return (
    <div className="space-y-6">
      <PageBreadcrumb items={[{ href: "/overview", label: "Overview" }, { label: "Workspaces" }]} />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">AI</div>
          <h1 className="text-3xl font-semibold tracking-tight">Workspaces</h1>
          <p className="max-w-3xl text-sm leading-6 text-muted-foreground lg:text-base">
            Concrete workspace instances created from workflow templates with optional instantiation parameters.
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
            <CardTitle>Workspace catalog</CardTitle>
            <CardDescription>Instantiated workspaces and the templates they were created from.</CardDescription>
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
            <EmptyState description="Workspace instances will appear here after they are created." title="No workspaces" />
          ) : (
            <div className="rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Template</TableHead>
                    <TableHead className="text-right">Parameters</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead>Updated</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((workspace) => (
                    <TableRow key={workspace.name}>
                      <TableCell className="font-medium">{workspace.name}</TableCell>
                      <TableCell>{workspace.workspace_template_name}</TableCell>
                      <TableCell className="text-right">{Object.keys(workspace.parameters ?? {}).length}</TableCell>
                      <TableCell className="text-sm text-muted-foreground">{formatDate(workspace.created_at)}</TableCell>
                      <TableCell className="text-sm text-muted-foreground">{formatDate(workspace.updated_at)}</TableCell>
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
