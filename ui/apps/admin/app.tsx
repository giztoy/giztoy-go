import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";

import { configureAdminClients } from "./lib/api";
import { AppRoutes } from "./router";

configureAdminClients();
promoteLegacyHashRoute();

const root = document.querySelector<HTMLElement>("#app");

if (root === null) {
  throw new Error("missing #app root");
}

createRoot(root).render(
  <StrictMode>
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  </StrictMode>,
);

function promoteLegacyHashRoute(): void {
  if (window.location.pathname !== "/" || window.location.hash === "") {
    return;
  }
  const hashPath = window.location.hash.slice(1).trim();
  if (hashPath === "") {
    return;
  }
  const normalizedPath = hashPath.startsWith("/") ? hashPath : `/${hashPath}`;
  window.history.replaceState(null, "", normalizedPath);
}
