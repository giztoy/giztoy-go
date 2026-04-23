import type { ReactNode } from "react";

import { Label } from "../../../packages/components/label";

export function FormField({
  children,
  description,
  label,
}: {
  children: ReactNode;
  description?: string;
  label: string;
}): JSX.Element {
  return (
    <div className="space-y-2 rounded-lg border bg-muted/20 p-4">
      <div className="space-y-1">
        <Label>{label}</Label>
        {description ? <p className="text-sm leading-6 text-muted-foreground">{description}</p> : null}
      </div>
      {children}
    </div>
  );
}
