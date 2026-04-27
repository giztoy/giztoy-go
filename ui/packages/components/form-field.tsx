import * as React from "react";

import { Label } from "./label";
import { cn } from "./utils";

interface FormFieldProps extends React.HTMLAttributes<HTMLDivElement> {
  description?: string;
  htmlFor?: string;
  label: string;
}

const FormField = React.forwardRef<HTMLDivElement, FormFieldProps>(
  ({ children, className, description, htmlFor, label, ...props }, ref) => (
    <div ref={ref} className={cn("space-y-2 rounded-lg border bg-muted/20 p-4", className)} {...props}>
      <div className="space-y-1">
        <Label htmlFor={htmlFor}>{label}</Label>
        {description ? <p className="text-sm leading-6 text-muted-foreground">{description}</p> : null}
      </div>
      {children}
    </div>
  ),
);
FormField.displayName = "FormField";

export { FormField };
