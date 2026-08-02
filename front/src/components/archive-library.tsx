import { useMemo, useRef, useState, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import {
	FileArchive,
	Info,
	RefreshCw,
	Search,
	SlidersHorizontal,
	X,
} from "lucide-react";
import type { Archive } from "@/models/archive";
import { displayArchiveName, formatDateTime, hostname } from "@/lib/format";
import { cn } from "@/lib/utils";
import { ArchiveDetailsDialog } from "@/components/archive-details-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { gsap, useGSAP } from "@/lib/motion";

interface Props {
	archives: Archive[];
	selectedArchive: string;
	onSelect: (id: string) => void;
	loading: boolean;
	error: string | null;
	onRefresh: () => Promise<void>;
}
export function ArchiveLibrary({
	archives,
	selectedArchive,
	onSelect,
	loading,
	error,
	onRefresh,
}: Props) {
	const libraryRef = useRef<HTMLElement>(null);
	const [query, setQuery] = useState("");
	const [tags, setTags] = useState<string[]>([]);
	const [details, setDetails] = useState<Archive | null>(null);
	const allTags = useMemo(
		() => [...new Set(archives.flatMap((a) => a.tags ?? []))].sort(),
		[archives],
	);
	const filtered = useMemo(
		() =>
			archives.filter((a) => {
				const q = query.toLowerCase();
				return (
					(!q ||
						[a.name, a.description, a.source_url, ...(a.tags ?? [])].some((v) =>
							v?.toLowerCase().includes(q),
						)) &&
					(!tags.length || tags.every((t) => a.tags?.includes(t)))
				);
			}),
		[archives, query, tags],
	);
	const clear = () => {
		setQuery("");
		setTags([]);
	};
	const update = (archive: Archive) => {
		setDetails(archive);
	};
	const visibleArchiveKey = filtered.map((archive) => archive.id).join(",");
	useGSAP(
		() => {
			const media = gsap.matchMedia();
			media.add("(prefers-reduced-motion: no-preference)", () => {
				const rows = libraryRef.current?.querySelectorAll("[data-archive-row]");
				if (rows?.length) {
					gsap.fromTo(
						rows,
						{ opacity: 0, y: 5 },
						{ opacity: 1, y: 0, duration: 0.18, stagger: 0.025 },
					);
				}
			});
			return () => media.revert();
		},
		{ scope: libraryRef, dependencies: [visibleArchiveKey] },
	);
	return (
		<aside
			ref={libraryRef}
			className="flex min-h-0 w-full flex-col bg-surface md:w-(--library-width) md:border-r"
			aria-label="Archive library"
		>
			<div className="space-y-3 border-b p-3 pr-12 md:pr-3">
				<div className="flex items-center justify-between">
					<div>
						<h1 className="font-semibold">Archives</h1>
						<p className="text-xs text-muted-foreground" aria-live="polite">
							{filtered.length} of {archives.length} captures
						</p>
					</div>
					<div className="flex">
						<Button
							variant="ghost"
							size="icon"
							onClick={() => void onRefresh()}
							disabled={loading}
							aria-label="Refresh archive list"
						>
							<RefreshCw className={cn("size-4", loading && "animate-spin")} />
						</Button>
					</div>
				</div>
				<div className="flex gap-2">
					<div className="relative min-w-0 flex-1">
						<Search className="pointer-events-none absolute left-3 top-3 size-4 text-muted-foreground" />
						<Input
							aria-label="Search archives"
							value={query}
							onChange={(e) => setQuery(e.target.value)}
							placeholder="Search captures"
							className="h-10 pl-9"
						/>
					</div>
					<Popover>
						<PopoverTrigger asChild>
							<Button
								variant="outline"
								size="icon"
								aria-label="Filter archives by tags"
								aria-pressed={tags.length > 0}
							>
								<SlidersHorizontal className="size-4" />
							</Button>
						</PopoverTrigger>
						<PopoverContent align="end" className="w-60">
							<p className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
								Filter by tag
							</p>
							<div className="max-h-60 space-y-1 overflow-y-auto">
								{allTags.map((tag) => (
									<button
										key={tag}
										type="button"
										aria-pressed={tags.includes(tag)}
										onClick={() =>
											setTags((p) =>
												p.includes(tag)
													? p.filter((x) => x !== tag)
													: [...p, tag],
											)
										}
										className={cn(
											"flex min-h-10 w-full items-center rounded-md px-2 text-left text-sm",
											tags.includes(tag)
												? "bg-accent text-accent-foreground"
												: "hover:bg-muted",
										)}
									>
										{tag}
									</button>
								))}
								{!allTags.length && (
									<p className="py-3 text-center text-sm text-muted-foreground">
										No tags yet
									</p>
								)}
							</div>
						</PopoverContent>
					</Popover>
				</div>
				{(query || tags.length > 0) && (
					<div className="flex items-center gap-1">
						<span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
							{tags.length
								? `${tags.length} tag filter${tags.length > 1 ? "s" : ""}`
								: "Search active"}
						</span>
						<Button variant="ghost" size="sm" onClick={clear}>
							<X className="size-3" />
							Clear
						</Button>
					</div>
				)}
			</div>
			<div className="min-h-0 flex-1 overflow-y-auto p-2" aria-busy={loading}>
				{loading && !archives.length ? (
					<LibrarySkeleton />
				) : error ? (
					<State
						title="Couldn’t refresh archives"
						message={error}
						action={
							<Button size="sm" onClick={() => void onRefresh()}>
								Try again
							</Button>
						}
					/>
				) : filtered.length ? (
					<div className="space-y-1">
						{filtered.map((a) => (
							<div key={a.id} data-archive-row className="group relative">
								<button
									type="button"
									onClick={() => onSelect(a.id)}
									className={cn(
										"flex min-h-16 w-full items-start gap-3 rounded-md p-3 pr-10 text-left transition-colors",
										a.id === selectedArchive
											? "bg-surface-subtle shadow-sm before:absolute before:inset-y-3 before:left-0 before:w-1 before:rounded-full before:bg-primary"
											: "hover:bg-muted/70",
									)}
								>
									<FileArchive className="mt-0.5 size-4 shrink-0 text-primary" />
									<span className="min-w-0 flex-1">
										<span className="block truncate text-sm font-medium">
											{displayArchiveName(a.name)}
										</span>
										<span className="block truncate font-mono text-[.7rem] text-muted-foreground">
											{hostname(a.source_url)}
										</span>
										<span className="mt-1 block text-[.7rem] text-muted-foreground">
											{formatDateTime(a.created_at)}
										</span>
										{a.tags?.length > 0 && (
											<span className="mt-1 flex gap-1">
												{a.tags.slice(0, 2).map((t) => (
													<Badge
														key={t}
														variant="secondary"
														className="h-4 max-w-20 truncate px-1 text-[.6rem]"
													>
														{t}
													</Badge>
												))}
												{a.tags.length > 2 && (
													<span className="text-[.65rem] text-muted-foreground">
														+{a.tags.length - 2}
													</span>
												)}
											</span>
										)}
									</span>
								</button>
								<Button
									type="button"
									variant="ghost"
									size="icon"
									className="absolute right-1 top-2"
									onClick={(e) => {
										e.stopPropagation();
										setDetails(a);
									}}
									aria-label={`Archive details for ${displayArchiveName(a.name)}`}
								>
									<Info className="size-4" />
								</Button>
							</div>
						))}
					</div>
				) : (
					<State
						title={
							archives.length ? "No matching captures" : "Your archive is empty"
						}
						message={
							archives.length
								? "Try clearing your search or tag filters."
								: "Create an archive to begin building your collection."
						}
						action={
							archives.length ? (
								<Button size="sm" variant="outline" onClick={clear}>
									Clear filters
								</Button>
							) : (
								<Button size="sm" asChild>
									<Link to="/create-archive">New archive</Link>
								</Button>
							)
						}
					/>
				)}
			</div>
			<ArchiveDetailsDialog
				archive={details}
				open={!!details}
				onOpenChange={(open) => !open && setDetails(null)}
				onDeleted={(id) => {
					if (id === selectedArchive) onSelect("");
					setDetails(null);
				}}
				onUpdated={update}
			/>
		</aside>
	);
}
function State({
	title,
	message,
	action,
}: {
	title: string;
	message: string;
	action: ReactNode;
}) {
	return (
		<div className="grid min-h-48 place-items-center p-5 text-center">
			<div className="space-y-2">
				<p className="font-medium">{title}</p>
				<p className="max-w-56 text-sm text-muted-foreground">{message}</p>
				{action}
			</div>
		</div>
	);
}
function LibrarySkeleton() {
	return (
		<div className="space-y-2 p-2">
			{[1, 2, 3, 4].map((i) => (
				<div key={i} className="h-16 animate-pulse rounded-md bg-muted" />
			))}
		</div>
	);
}
