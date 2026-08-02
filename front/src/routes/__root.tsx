import { createRootRoute } from "@tanstack/react-router";
import { ThemeProvider } from "@/components/theme-provider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import { AppShell } from "@/components/app-shell";
const RootLayout = () => (
	<ThemeProvider>
		<TooltipProvider>
			<AppShell />
			<Toaster position="bottom-right" richColors />
		</TooltipProvider>
	</ThemeProvider>
);
export const Route = createRootRoute({ component: RootLayout });
