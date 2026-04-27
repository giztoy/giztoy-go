import * as React from "react";

import { Card, CardContent, CardHeader, CardTitle } from "./card";
import { cn } from "./utils";

type DetailBlockItem = [label: string, value: string | undefined];

interface DetailBlockProps extends React.HTMLAttributes<HTMLDivElement> {
  items: DetailBlockItem[];
  title: string;
}

const DetailBlock = React.forwardRef<HTMLDivElement, DetailBlockProps>(
  ({ className, items, title, ...props }, ref) => (
    <Card ref={ref} className={className} {...props}>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {items.map(([label, value]) => (
          <div className="flex items-start justify-between gap-4 text-sm" key={label}>
            <span className="text-muted-foreground">{label}</span>
            <span className={cn("max-w-[16rem] text-right text-foreground")}>{formatDetailValue(value)}</span>
          </div>
        ))}
      </CardContent>
    </Card>
  ),
);
DetailBlock.displayName = "DetailBlock";

function formatDetailValue(value: string | undefined): string {
  if (value === undefined || value === "") {
    return "—";
  }
  if (!isDateTimeLike(value)) {
    return value;
  }
  const time = Date.parse(value);
  return Number.isNaN(time) ? value : new Date(time).toLocaleString();
}

function isDateTimeLike(value: string): boolean {
  return value.includes("T") || value.endsWith("Z");
}

export { DetailBlock };
export type { DetailBlockItem };
