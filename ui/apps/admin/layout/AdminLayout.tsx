import { Outlet } from "react-router-dom";

import { AppSidebar } from "./AppSidebar";

export function AdminLayout(): JSX.Element {
  return (
    <div className="min-h-screen bg-muted/30">
      <div className="grid min-h-screen lg:grid-cols-[248px_minmax(0,1fr)]">
        <AppSidebar />
        <main className="min-w-0">
          <div className="mx-auto flex w-full max-w-[1400px] flex-col gap-8 px-6 py-6 lg:px-10 lg:py-10">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
