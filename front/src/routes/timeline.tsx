import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { Search } from "lucide-react";
import { apiClient } from "@/lib/api";
import type { Archive, GetArchivesResponse } from "@/models/archive";
import { ArchiveViewer } from "@/components/archive-viewer";
import { ArchiveTimeline } from "@/components/archive-timeline";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { paddedArchiveRange } from "@/lib/timeline";
export const Route = createFileRoute("/timeline")({ component: Timeline });
function Timeline() {
	const [archives, setArchives] = useState<Archive[]>([]);
	const [query, setQuery] = useState("");
	const [submitted, setSubmitted] = useState("");
	const [selected, setSelected] = useState("");
	const [range, setRange] = useState<{ start: Date; end: Date } | null>(null);
	const [error, setError] = useState<string | null>(null);
	useEffect(() => {
		apiClient
			.get<GetArchivesResponse>("/archives")
			.then((d) =>
				setArchives(
					[...d.archives].sort(
						(a, b) => +new Date(a.created_at) - +new Date(b.created_at),
					),
				),
			)
			.catch((e) =>
				setError(e instanceof Error ? e.message : "Unable to load archives"),
			);
	}, []);
	const matches = useMemo(
		() =>
			archives.filter(
				(a) =>
					!submitted ||
					a.source_url.toLowerCase().includes(submitted.toLowerCase()),
			),
		[archives, submitted],
	);
	const fallback = useMemo(() => {
		const end = new Date();
		const start = new Date(end);
		start.setDate(end.getDate() - 7);
		return paddedArchiveRange(matches) ?? { start, end };
	}, [matches]);
	const active = range ?? fallback;
	return (
		<div className="flex h-full min-h-0 flex-col overflow-y-auto">
			<div className="mx-auto w-full max-w-6xl p-(--content-gutter)">
				<header className="mb-4">
					<p className="text-sm font-semibold uppercase tracking-[.16em] text-primary">
						Capture history
					</p>
					<h1 className="mt-1 text-3xl font-semibold">Timeline</h1>
				</header>
				<form
					className="flex gap-2"
					onSubmit={(e) => {
						e.preventDefault();
						setSubmitted(query);
						setRange(null);
					}}
				>
					<Input
						aria-label="Search captures by URL"
						value={query}
						onChange={(e) => setQuery(e.target.value)}
						placeholder="Filter by source URL"
					/>
					<Button type="submit">
						<Search className="size-4" />
						Search
					</Button>
				</form>
				<p className="mt-2 text-sm text-muted-foreground" aria-live="polite">
					{matches.length} capture{matches.length === 1 ? "" : "s"} found
				</p>
				{error && (
					<p role="alert" className="mt-3 text-sm text-destructive">
						{error}
					</p>
				)}
			</div>
			<div className="mx-auto flex min-h-0 w-full max-w-6xl flex-1 flex-col gap-3 px-(--content-gutter) pb-3">
				<div className="min-h-[clamp(22rem,58dvh,42rem)] flex-1">
					<ArchiveViewer archive={archives.find((a) => a.id === selected)} />
				</div>
				<ArchiveTimeline
					archives={matches}
					selectedArchive={selected}
					rangeStart={active.start}
					rangeEnd={active.end}
					onSelect={setSelected}
					onRangeChange={(start, end) => setRange({ start, end })}
				/>
			</div>
		</div>
	);
}
