export function EmptyState({ description, title }: { description: string; title: string }): JSX.Element {
  return (
    <div className="flex min-h-56 flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border bg-muted/20 px-6 py-10 text-center">
      <div className="text-base font-medium">{title}</div>
      <p className="max-w-md text-sm leading-6 text-muted-foreground">{description}</p>
    </div>
  );
}
