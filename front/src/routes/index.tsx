import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useState } from "react";
import { PanelLeft } from "lucide-react";
import { apiClient } from "@/lib/api";
import type { Archive, GetArchivesResponse } from "@/models/archive";
import { ArchiveLibrary } from "@/components/archive-library";
import { ArchiveViewer } from "@/components/archive-viewer";
import { Button } from "@/components/ui/button";
import {
	Sheet,
	SheetContent,
	SheetHeader,
	SheetTitle,
	SheetTrigger,
} from "@/components/ui/sheet";
export const Route = createFileRoute("/")({
	loader: async () =>
		(await apiClient.get<GetArchivesResponse>("/archives")).archives,
	component: Index,
});
function Index() {
	const initial = Route.useLoaderData();
	const [archives, setArchives] = useState<Archive[]>(initial);
	const [selected, setSelected] = useState("");
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [drawer, setDrawer] = useState(false);
	const refresh = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			setArchives(
				(await apiClient.get<GetArchivesResponse>("/archives")).archives,
			);
		} catch (e) {
			setError(e instanceof Error ? e.message : "Unable to load archives");
		} finally {
			setLoading(false);
		}
	}, []);
	const choose = (id: string) => {
		setSelected(id);
		setDrawer(false);
	};
	const selectedArchive = archives.find((a) => a.id === selected);
	const library = (
		<ArchiveLibrary
			archives={archives}
			selectedArchive={selectedArchive?.id ?? ""}
			onSelect={choose}
			loading={loading}
			error={error}
			onRefresh={refresh}
		/>
	);
	return (
		<div className="flex h-[calc(100dvh-var(--app-header-height))] min-h-0">
			<div className="hidden min-h-0 md:flex">{library}</div>
			<Sheet open={drawer} onOpenChange={setDrawer}>
				<SheetTrigger asChild>
					<Button
						className="fixed bottom-4 left-4 z-30 shadow-panel md:hidden"
						size="sm"
					>
						<PanelLeft className="size-4" />
						Library
					</Button>
				</SheetTrigger>
				<SheetContent
					side="left"
					className="w-[min(22rem,calc(100vw-1rem))] p-0"
				>
					<SheetHeader className="sr-only">
						<SheetTitle>Archive library</SheetTitle>
					</SheetHeader>
					{library}
				</SheetContent>
			</Sheet>
			<div className="min-w-0 flex-1 p-3 md:p-4">
				<ArchiveViewer archive={selectedArchive} />
			</div>
		</div>
	);
}
