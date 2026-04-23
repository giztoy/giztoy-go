import { Card, CardContent, CardHeader, CardTitle } from "../../../packages/components/card";

import { formatValue } from "../lib/format";

export function DetailBlock({
  items,
  title,
}: {
  items: Array<[string, string | undefined]>;
  title: string;
}): JSX.Element {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {items.map(([label, value]) => (
          <div className="flex items-start justify-between gap-4 text-sm" key={label}>
            <span className="text-muted-foreground">{label}</span>
            <span className="max-w-[16rem] text-right text-foreground">{formatValue(value)}</span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
