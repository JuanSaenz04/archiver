import { Link, Outlet, useRouterState } from "@tanstack/react-router";
import { useState } from "react";
import {
	Archive,
	CalendarRange,
	List,
	Menu,
	Moon,
	Plus,
	Sun,
} from "lucide-react";
import { JobsSheet } from "@/components/jobs-sheet";
import { ModeToggle } from "@/components/mode-toggle";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSub,
	DropdownMenuSubContent,
	DropdownMenuSubTrigger,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useTheme } from "@/components/use-theme";

const sectionFor = (path: string) =>
	path === "/timeline"
		? "Timeline"
		: path === "/create-archive"
			? "New archive"
			: "Archive library";
export function AppShell() {
	const path = useRouterState({ select: (state) => state.location.pathname });
	const section = sectionFor(path);
	const [mobileJobsOpen, setMobileJobsOpen] = useState(false);
	const { setTheme } = useTheme();
	return (
		<div className="flex h-dvh min-h-dvh flex-col bg-background">
			<header
				className="sticky top-0 z-40 flex min-h-(--app-header-height) shrink-0 items-center justify-between border-b bg-background/95 px-(--content-gutter) backdrop-blur supports-backdrop-filter:bg-background/80"
				style={{ paddingTop: "env(safe-area-inset-top)" }}
			>
				<div className="flex min-w-0 items-center gap-3">
					<Link
						to="/"
						className="flex shrink-0 items-center gap-2 font-semibold tracking-tight"
					>
						<span className="grid size-8 place-items-center rounded-md bg-primary text-primary-foreground">
							<Archive className="size-4" />
						</span>
						<span className="hidden sm:inline">Archiver</span>
					</Link>
					<span className="h-5 border-l" />
					<span
						className="truncate text-sm text-muted-foreground"
						aria-current="page"
					>
						{section}
					</span>
				</div>
				<nav aria-label="Application" className="flex items-center gap-1">
					<div className="hidden items-center gap-1 sm:flex">
						<JobsSheet />
						<Link to="/timeline" activeProps={{ "aria-current": "page" }}>
							<Button
								variant={path === "/timeline" ? "secondary" : "ghost"}
								size="icon"
								aria-label="Open timeline"
							>
								<CalendarRange className="size-4" />
							</Button>
						</Link>
						<ModeToggle />
					</div>
					<Link to="/create-archive">
						<Button size="sm" className="gap-1.5">
							<Plus className="size-4" />
							<span className="hidden xs:inline">New archive</span>
							<span className="xs:hidden">New</span>
						</Button>
					</Link>
					<div className="sm:hidden">
						<JobsSheet
							open={mobileJobsOpen}
							onOpenChange={setMobileJobsOpen}
							showTrigger={false}
						/>
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button
									variant="ghost"
									size="icon"
									aria-label="More navigation options"
								>
									<Menu className="size-5" />
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end">
								<DropdownMenuItem asChild>
									<Link to="/timeline">
										<CalendarRange className="size-4" />
										Timeline
									</Link>
								</DropdownMenuItem>
								<DropdownMenuItem
									onSelect={() => {
										setMobileJobsOpen(true);
									}}
								>
									<List className="size-4" />
									Jobs
								</DropdownMenuItem>
								<DropdownMenuSub>
									<DropdownMenuSubTrigger>
										<Sun className="size-4" />
										Theme
									</DropdownMenuSubTrigger>
									<DropdownMenuSubContent>
										<DropdownMenuItem onSelect={() => setTheme("light")}>
											<Sun className="size-4" />
											Light
										</DropdownMenuItem>
										<DropdownMenuItem onSelect={() => setTheme("dark")}>
											<Moon className="size-4" />
											Dark
										</DropdownMenuItem>
										<DropdownMenuItem onSelect={() => setTheme("system")}>
											System
										</DropdownMenuItem>
									</DropdownMenuSubContent>
								</DropdownMenuSub>
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</nav>
			</header>
			<main className="min-h-0 flex-1 overflow-y-auto">
				<Outlet />
			</main>
		</div>
	);
}
