import type { Archive } from "@/models/archive";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import {
	dateInputValue,
	displayArchiveName,
	formatDate,
	formatDateTime,
	hostname,
} from "@/lib/format";
import { cn } from "@/lib/utils";
import { paddedArchiveRange } from "@/lib/timeline";
import { gsap, useGSAP } from "@/lib/motion";
interface Props {
	archives: Archive[];
	selectedArchive: string;
	rangeStart: Date;
	rangeEnd: Date;
	onSelect: (id: string) => void;
	onRangeChange: (start: Date, end: Date) => void;
}
export function ArchiveTimeline({
	archives,
	selectedArchive,
	rangeStart,
	rangeEnd,
	onSelect,
	onRangeChange,
}: Props) {
	const trackRef = useRef<HTMLDivElement>(null);
	const previousPositions = useRef(new Map<string, string>());
	const previousVisibility = useRef(new Map<string, boolean>());
	const previousRange = useRef({ start: rangeStart, end: rangeEnd });
	const presets: [string, number | "all"][] = [
		["7 days", 7],
		["30 days", 30],
		["1 year", 365],
		["All", "all"],
	];
	const preset = (days: number | "all") => {
		const end = new Date();
		end.setHours(23, 59, 59, 999);
		if (days === "all" && archives.length) {
			const allRange = paddedArchiveRange(archives);
			if (allRange) onRangeChange(allRange.start, allRange.end);
			return;
		}
		const start = new Date(end);
		start.setDate(start.getDate() - Number(days));
		start.setHours(0, 0, 0, 0);
		onRangeChange(start, end);
	};
	const start = rangeStart.getTime(),
		duration = Math.max(1, rangeEnd.getTime() - start);
	const visible = archives.filter((a) => {
		const time = new Date(a.created_at).getTime();
		return time >= start && time <= rangeEnd.getTime();
	});
	const visibleIds = new Set(visible.map((archive) => archive.id));
	const markerKey = visible
		.map((archive) => `${archive.id}:${archive.created_at}`)
		.join(",");
	const currentMarkers = visible.map((archive, index) => ({
		archive,
		offset: ((index % 3) - 1) * 8,
	}));
	const [markerState, setMarkerState] = useState({
		key: markerKey,
		current: currentMarkers,
		rendered: currentMarkers,
	});
	if (markerState.key !== markerKey) {
		const exiting = markerState.current.filter(
			({ archive }) => !visibleIds.has(archive.id),
		);
		setMarkerState({
			key: markerKey,
			current: currentMarkers,
			rendered: [...currentMarkers, ...exiting],
		});
	}
	useGSAP(
		() => {
			const markers = trackRef.current?.querySelectorAll<HTMLButtonElement>(
				"[data-timeline-marker]",
			);
			const reduceMotion = window.matchMedia(
				"(prefers-reduced-motion: reduce)",
			).matches;
			const timeline = gsap.timeline({
				onComplete: () =>
					setMarkerState((state) =>
						state.key === markerKey
							? { ...state, rendered: state.current }
							: state,
					),
			});
			markers?.forEach((marker) => {
				const id = marker.dataset.markerId;
				const target = marker.dataset.markerPosition;
				const isVisible = marker.dataset.markerVisible === "true";
				if (!id || !target) return;

				const time = Number(marker.dataset.markerTime);
				const oldRange = previousRange.current;
				const initialPosition =
					time < oldRange.start.getTime()
						? "-4%"
						: time > oldRange.end.getTime()
							? "104%"
							: target;
				const previous =
					previousPositions.current.get(id) ?? initialPosition;
				const wasVisible =
					previousVisibility.current.get(id) ?? previous === target;

				timeline.fromTo(
					marker,
					{ autoAlpha: wasVisible ? 1 : 0, left: previous },
					{
						autoAlpha: isVisible ? 1 : 0,
						duration: reduceMotion ? 0 : 0.32,
						ease: "power2.inOut",
						left: target,
						overwrite: "auto",
					},
					0,
				);
				previousPositions.current.set(id, target);
				previousVisibility.current.set(id, isVisible);
			});

			previousRange.current = { start: rangeStart, end: rangeEnd };
			return () => timeline.kill();
		},
		{ scope: trackRef, dependencies: [markerKey, rangeStart, rangeEnd] },
	);
	return (
		<section
			className="rounded-lg border bg-surface p-3 md:p-4"
			aria-label="Capture timeline"
		>
			<div className="flex flex-wrap gap-2">
				<div className="flex max-w-full gap-1 overflow-x-auto">
					{presets.map(([label, value]) => (
						<Button
							key={label}
							size="sm"
							variant="outline"
							onClick={() => preset(value)}
						>
							{label}
						</Button>
					))}
				</div>
				<div className="grid w-full gap-2 sm:ml-auto sm:w-auto sm:grid-cols-2">
					<label className="text-xs text-muted-foreground">
						From
						<Input
							type="date"
							className="mt-1 w-full"
							value={dateInputValue(rangeStart)}
							onChange={(e) => {
								const d = new Date(`${e.target.value}T00:00:00`);
								if (!Number.isNaN(+d) && d <= rangeEnd)
									onRangeChange(d, rangeEnd);
							}}
						/>
					</label>
					<label className="text-xs text-muted-foreground">
						To
						<Input
							type="date"
							className="mt-1 w-full"
							value={dateInputValue(rangeEnd)}
							onChange={(e) => {
								const d = new Date(`${e.target.value}T23:59:59`);
								if (!Number.isNaN(+d) && d >= rangeStart)
									onRangeChange(rangeStart, d);
							}}
						/>
					</label>
				</div>
			</div>
			<div className="mt-4 hidden md:block">
				<div ref={trackRef} className="relative h-14 px-4">
					<div className="absolute inset-x-0 top-1/2 h-px bg-border" />
					{markerState.rendered.map(({ archive: a, offset }) => {
						const isVisible = visibleIds.has(a.id);
						const ratio = (new Date(a.created_at).getTime() - start) / duration;
						const position =
							ratio < 0
								? "-4%"
								: ratio > 1
									? "104%"
									: `${(0.04 + ratio * 0.92) * 100}%`;
						return (
							<Tooltip key={a.id}>
								<TooltipTrigger asChild>
									<button
										data-timeline-marker
										data-marker-id={a.id}
										data-marker-position={position}
										data-marker-time={new Date(a.created_at).getTime()}
										data-marker-visible={isVisible}
										type="button"
										onClick={() => onSelect(a.id)}
										onKeyDown={(e) => {
											if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
												e.preventDefault();
												const i =
													visible.indexOf(a) +
													(e.key === "ArrowRight" ? 1 : -1);
												if (visible[i]) onSelect(visible[i].id);
											}
										}}
										className={cn(
											"absolute top-1/2 grid size-9 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full transition-[margin-top,background-color] duration-200 focus-visible:z-10",
											!isVisible && "pointer-events-none",
											a.id === selectedArchive
												? "bg-primary text-primary-foreground"
												: "bg-surface-raised ring-1 ring-border hover:bg-accent",
										)}
										style={{ left: position, marginTop: offset }}
										tabIndex={isVisible ? 0 : -1}
										aria-hidden={!isVisible}
										aria-label={`${displayArchiveName(a.name)}, ${formatDateTime(a.created_at)}`}
									>
										<span className="size-2 rounded-full bg-current" />
									</button>
								</TooltipTrigger>
								<TooltipContent side="top" sideOffset={6}>
									<p className="max-w-64 truncate font-medium">
										{displayArchiveName(a.name)}
									</p>
									<p className="max-w-64 truncate opacity-80">
										{hostname(a.source_url)} · {formatDateTime(a.created_at)}
									</p>
								</TooltipContent>
							</Tooltip>
						);
					})}
				</div>
				<div className="flex justify-between text-xs text-muted-foreground">
					<span>{formatDate(rangeStart)}</span>
					<span aria-live="polite">{visible.length} captures in range</span>
					<span>{formatDate(rangeEnd)}</span>
				</div>
			</div>
			<div className="mt-4 space-y-1 md:hidden" aria-live="polite">
				{visible.length ? (
					visible.map((a) => (
						<button
							key={a.id}
							type="button"
							onClick={() => onSelect(a.id)}
							className={cn(
								"flex min-h-14 w-full items-center gap-3 rounded-md p-3 text-left",
								a.id === selectedArchive
									? "bg-surface-subtle ring-1 ring-primary/30"
									: "bg-muted/50",
							)}
						>
							<span className="size-2 shrink-0 rounded-full bg-primary" />
							<span className="min-w-0 flex-1">
								<span className="block truncate text-sm font-medium">
									{displayArchiveName(a.name)}
								</span>
								<span className="block truncate font-mono text-xs text-muted-foreground">
									{hostname(a.source_url)}
								</span>
							</span>
							<time className="shrink-0 text-xs text-muted-foreground">
								{formatDateTime(a.created_at)}
							</time>
						</button>
					))
				) : (
					<p className="py-4 text-center text-sm text-muted-foreground">
						No captures in this range.
					</p>
				)}
			</div>
		</section>
	);
}
