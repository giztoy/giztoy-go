import * as React from "react";

import { cn } from "./utils";

interface EmptyStateProps extends React.HTMLAttributes<HTMLDivElement> {
  description: string;
  title: string;
}

const EmptyState = React.forwardRef<HTMLDivElement, EmptyStateProps>(
  ({ className, description, title, ...props }, ref) => (
    <div
      ref={ref}
      className={cn(
        "flex min-h-56 flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border bg-muted/20 px-6 py-10 text-center",
        className,
      )}
      {...props}
    >
      <div className="text-base font-medium">{title}</div>
      <p className="max-w-md text-sm leading-6 text-muted-foreground">{description}</p>
    </div>
  ),
);
EmptyState.displayName = "EmptyState";

export { EmptyState };
