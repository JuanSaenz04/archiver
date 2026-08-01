import "./index.css";
import { StrictMode } from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider, createRouter } from "@tanstack/react-router";

// Import the generated route tree
import { routeTree } from "./routeTree.gen";
import { loadRuntimeConfig } from "./lib/runtime-config";
import { RuntimeConfigProvider } from "./components/runtime-config-provider";

// Create a new router instance
const router = createRouter({ routeTree });

// Register the router instance for type safety
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

async function renderApp() {
  const rootElement = document.getElementById("root")!;
  if (rootElement.innerHTML) return;

  const root = ReactDOM.createRoot(rootElement);
  try {
    const config = await loadRuntimeConfig();
    root.render(
      <StrictMode>
        <RuntimeConfigProvider config={config}>
          <RouterProvider router={router} />
        </RuntimeConfigProvider>
      </StrictMode>,
    );
  } catch (error) {
    console.error(error);
    root.render(
      <main className="p-6 font-sans">
        Unable to load Archiver configuration. Check the API server settings.
      </main>,
    );
  }
}

void renderApp();
