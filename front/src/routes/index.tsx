import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { PanelLeft } from "lucide-react";
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
import { archivesQueryOptions } from "@/lib/queries";
import { queryClient } from "@/lib/query-client";
export const Route = createFileRoute("/")({
	loader: () => queryClient.ensureQueryData(archivesQueryOptions),
	component: Index,
});
function Index() {
	const { data: archives = [], error, isFetching, refetch } = useQuery(
		archivesQueryOptions,
	);
	const [selected, setSelected] = useState("");
	const [drawer, setDrawer] = useState(false);
	const refresh = async () => {
		await refetch();
	};
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
			loading={isFetching}
			error={error?.message ?? null}
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
