import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
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
import {
	archivePagesQueryOptions,
	archiveTagsQueryOptions,
} from "@/lib/queries";
import { queryClient } from "@/lib/query-client";
export const Route = createFileRoute("/")({
	loader: () =>
		Promise.all([
			queryClient.ensureInfiniteQueryData(
				archivePagesQueryOptions({ search: "", tags: [] }),
			),
			queryClient.ensureQueryData(archiveTagsQueryOptions),
		]),
	component: Index,
});
function Index() {
	const [query, setQuery] = useState("");
	const [debouncedQuery, setDebouncedQuery] = useState("");
	const [tags, setTags] = useState<string[]>([]);
	useEffect(() => {
		const timeout = window.setTimeout(() => setDebouncedQuery(query), 300);
		return () => window.clearTimeout(timeout);
	}, [query]);
	const {
		data,
		error,
		isFetching,
		isFetchingNextPage,
		fetchNextPage,
		hasNextPage,
		refetch,
	} = useInfiniteQuery(
		archivePagesQueryOptions({ search: debouncedQuery, tags }),
	);
	const { data: availableTags = [] } = useQuery(archiveTagsQueryOptions);
	const archives = data?.pages.flatMap((page) => page.archives) ?? [];
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
			availableTags={availableTags}
			query={query}
			onQueryChange={setQuery}
			tags={tags}
			onTagsChange={setTags}
			selectedArchive={selectedArchive?.id ?? ""}
			onSelect={choose}
			loading={isFetching}
			error={error?.message ?? null}
			onRefresh={refresh}
			hasMore={hasNextPage}
			loadingMore={isFetchingNextPage}
			onLoadMore={() => fetchNextPage()}
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
