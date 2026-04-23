export function ErrorBanner({ message }: { message: string }): JSX.Element {
  return (
    <div className="rounded-lg border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
      {message}
    </div>
  );
}

export function NoticeBanner({ message, tone }: { message: string; tone: "error" | "success" }): JSX.Element {
  if (tone === "error") {
    return <ErrorBanner message={message} />;
  }
  return (
    <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
      {message}
    </div>
  );
}
