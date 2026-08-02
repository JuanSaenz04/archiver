import { useRef, useState } from "react";
import { ExternalLink, FileArchive, Loader2 } from "lucide-react";
import type { Archive } from "@/models/archive";
import { useRuntimeConfig } from "@/lib/runtime-config";
import { displayArchiveName, formatDateTime, hostname } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { gsap, useGSAP } from "@/lib/motion";
interface Props {
	archive?: Archive;
}
export function ArchiveViewer({ archive }: Props) {
	const { replay_origin: origin } = useRuntimeConfig();
	const [loadedId, setLoadedId] = useState("");
	const panel = useRef<HTMLElement>(null);
	useGSAP(
		() => {
			const media = gsap.matchMedia();
			media.add("(prefers-reduced-motion: no-preference)", () => {
				if (panel.current)
					gsap.fromTo(
						panel.current,
						{ opacity: 0.65, y: 6 },
						{ opacity: 1, y: 0 },
					);
			});
			return () => media.revert();
		},
		{ scope: panel, dependencies: [archive?.id] },
	);
	if (!archive)
		return (
			<section
				ref={panel}
				className="grid h-full min-h-72 place-items-center rounded-lg bg-surface-subtle p-6 text-center"
			>
				<div>
					<FileArchive className="mx-auto mb-3 size-8 text-primary" />
					<h2 className="font-semibold">Choose a capture to inspect</h2>
					<p className="mt-1 max-w-sm text-sm text-muted-foreground">
						Select an archive from the library, or create a new capture to get
						started.
					</p>
				</div>
			</section>
		);
	const viewerUrl = new URL("/viewer.html", origin);
	viewerUrl.searchParams.set("source", `/archives/${archive.id}`);
	const loading = loadedId !== archive.id;
	return (
		<section
			ref={panel}
			className="flex h-full min-h-0 flex-col overflow-hidden rounded-lg border bg-surface shadow-panel"
		>
			<div className="flex min-h-14 shrink-0 items-center justify-between gap-3 border-b px-3">
				<div className="min-w-0">
					<h2 className="truncate text-sm font-semibold">
						{displayArchiveName(archive.name)}
					</h2>
					<p className="truncate font-mono text-[.7rem] text-muted-foreground">
						{hostname(archive.source_url)} ·{" "}
						{formatDateTime(archive.created_at)}
					</p>
				</div>
				{archive.source_url && (
					<Button variant="ghost" size="icon" asChild>
						<a
							href={archive.source_url}
							target="_blank"
							rel="noopener noreferrer"
							aria-label="Open live source"
						>
							<ExternalLink className="size-4" />
						</a>
					</Button>
				)}
			</div>
			<div className="relative min-h-0 flex-1">
				{loading && (
					<div
						className="absolute inset-0 z-10 grid place-items-center bg-surface/80"
						aria-live="polite"
					>
						<span className="flex items-center gap-2 text-sm text-muted-foreground">
							<Loader2 className="size-4 animate-spin" />
							Loading replay…
						</span>
					</div>
				)}
				<iframe
					key={viewerUrl.toString()}
					src={viewerUrl.toString()}
					onLoad={() => setLoadedId(archive.id)}
					className="h-full w-full border-0"
					title={`Archive replay: ${displayArchiveName(archive.name)}`}
				/>
			</div>
		</section>
	);
}
