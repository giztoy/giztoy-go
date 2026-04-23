import { Boxes, HardDrive, LayoutDashboard, MemoryStick } from "lucide-react";
import { NavLink } from "react-router-dom";

import { buttonVariants } from "../../../packages/components/button";
import { Card, CardContent } from "../../../packages/components/card";
import { cn } from "../../../packages/components/utils";

const linkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    "h-11 w-full justify-start gap-3 rounded-xl px-4 text-sm font-medium transition-colors",
    isActive
      ? "bg-primary text-primary-foreground shadow-sm hover:bg-primary/90 hover:text-primary-foreground"
      : "text-muted-foreground hover:bg-muted hover:text-foreground",
  );

export function AppSidebar(): JSX.Element {
  return (
    <aside className="border-r bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
      <div className="sticky top-0 flex h-screen w-[248px] flex-col">
        <div className="px-6 py-6">
          <Card className="rounded-2xl bg-muted/30">
            <CardContent className="p-4">
              <div className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">GizClaw</div>
              <div className="mt-1 text-lg font-semibold tracking-tight text-foreground">Admin Console</div>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">Overview, devices, firmware, and system surfaces.</p>
            </CardContent>
          </Card>
        </div>

        <nav className="flex flex-1 flex-col gap-1 px-3">
          <NavLink className={({ isActive }) => cn(buttonVariants({ variant: "ghost" }), linkClass({ isActive }))} end to="/overview">
            <LayoutDashboard className="size-4" />
            Overview
          </NavLink>
          <NavLink className={({ isActive }) => cn(buttonVariants({ variant: "ghost" }), linkClass({ isActive }))} to="/devices">
            <Boxes className="size-4" />
            Devices
          </NavLink>
          <NavLink className={({ isActive }) => cn(buttonVariants({ variant: "ghost" }), linkClass({ isActive }))} to="/firmware">
            <HardDrive className="size-4" />
            Firmware
          </NavLink>
          <NavLink className={({ isActive }) => cn(buttonVariants({ variant: "ghost" }), linkClass({ isActive }))} end to="/memory">
            <MemoryStick className="size-4" />
            Memory
          </NavLink>
        </nav>

        <div className="px-6 pb-6 pt-4">
          <Card className="rounded-xl bg-muted/20 shadow-none">
            <CardContent className="px-4 py-3 text-xs leading-5 text-muted-foreground">
              First-level navigation only. Details stay inside each resource page.
            </CardContent>
          </Card>
        </div>
      </div>
    </aside>
  );
}
