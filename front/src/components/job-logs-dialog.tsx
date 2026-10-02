import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowDown } from "lucide-react";
import type { Job } from "@/models/job";
import { cn } from "@/lib/utils";
import { compactId, hostname } from "@/lib/format";
import { queryKeys } from "@/lib/queries";
import { watchJobLogs, type LogEntry } from "@/lib/job-logs";
import { StatusPill } from "@/components/status-pill";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";

const MAX_LINES = 2000;
const JOB_STATUSES = ["pending", "running", "completed", "failed"];
const timeFormat = new Intl.DateTimeFormat(undefined, {
	hour: "2-digit",
	minute: "2-digit",
	second: "2-digit",
	hour12: false,
});

type Level = "info" | "warn" | "error" | "system";
interface Line { id: string; time: string; tag: string; level: Level; message: string; details?: string }

// browsertrix-crawler writes JSON lines; anything else is shown as plain text.
function toLine(entry: LogEntry): Line {
	const date = new Date(entry.time);
	const time = Number.isNaN(date.getTime()) ? "" : timeFormat.format(date);
	const base = { id: entry.id, time };
	if (entry.source === "system") return { ...base, tag: "archiver", level: "system", message: entry.text };
	const fallbackLevel: Level = entry.source === "stderr" ? "warn" : "info";
	if (entry.text.startsWith("{")) {
		try {
			const parsed = JSON.parse(entry.text) as { logLevel?: string; context?: string; message?: string; details?: unknown };
			if (typeof parsed.message === "string") {
				const level: Level = parsed.logLevel === "error" || parsed.logLevel === "fatal" ? "error" : parsed.logLevel === "warn" ? "warn" : "info";
				const hasDetails = parsed.details && typeof parsed.details === "object" && Object.keys(parsed.details).length > 0;
				return { ...base, tag: parsed.context ?? entry.source, level, message: parsed.message, details: hasDetails ? JSON.stringify(parsed.details) : undefined };
			}
		} catch {
			// Not JSON after all; fall through to plain text.
		}
	}
	return { ...base, tag: entry.source, level: fallbackLevel, message: entry.text };
}

const levelStyles: Record<Level, { tag: string; message: string; row: string }> = {
	info: { tag: "text-muted-foreground", message: "", row: "" },
	system: { tag: "text-primary", message: "font-medium text-primary", row: "bg-primary/5" },
	warn: { tag: "text-warning", message: "", row: "bg-warning/10" },
	error: { tag: "text-destructive", message: "text-destructive", row: "bg-destructive/10" },
};

type Connection = "connecting" | "live" | "reconnecting" | "error" | "ended";
const connectionLabels: Record<Connection, string> = {
	connecting: "Connecting…",
	live: "Live",
	reconnecting: "Reconnecting…",
	error: "Unable to open logs. Close and reopen to retry.",
	ended: "Stream ended",
};

export function JobLogsDialog({ job, onClose }: { job: Job; onClose: () => void }) {
	const [lines, setLines] = useState<Line[]>([]);
	const [jobStatus, setJobStatus] = useState(job.status);
	const [connection, setConnection] = useState<Connection>("connecting");
	const [notice, setNotice] = useState("");
	const [following, setFollowing] = useState(true);
	const viewport = useRef<HTMLDivElement>(null);
	const queryClient = useQueryClient();

	useEffect(() => watchJobLogs(job.id, {
		onLines: batch => setLines(previous => [...previous, ...batch.map(toLine)].slice(-MAX_LINES)),
		onStatus: value => {
			// Job statuses may carry a suffix such as "failed · Logs expired or unavailable".
			const [state, detail] = value.split(" · ");
			if (JOB_STATUSES.includes(state)) {
				setJobStatus(state);
				if (detail) setNotice(detail);
				setConnection(current => (current === "connecting" || current === "reconnecting" ? "live" : current));
			} else if (value === "Connected") setConnection("live");
			else if (value.startsWith("Connection interrupted")) setConnection("reconnecting");
			else if (value.startsWith("Unable")) setConnection("error");
		},
		onNotice: setNotice,
		onDone: () => {
			setConnection("ended");
			void queryClient.invalidateQueries({ queryKey: queryKeys.jobs });
		},
	}), [job.id, queryClient]);

	useEffect(() => {
		if (following && viewport.current) viewport.current.scrollTop = viewport.current.scrollHeight;
	}, [lines, following]);

	const handleScroll = () => {
		const el = viewport.current;
		if (el) setFollowing(el.scrollHeight - el.scrollTop - el.clientHeight < 40);
	};

	return (
		<Dialog open onOpenChange={open => { if (!open) onClose(); }}>
			<DialogContent className="flex h-[min(85vh,52rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-5xl">
				<DialogHeader className="shrink-0 gap-1.5 border-b px-5 py-4 pr-12">
					<div className="flex items-center gap-2.5">
						<DialogTitle>Crawl logs</DialogTitle>
						<StatusPill status={jobStatus} />
					</div>
					<DialogDescription className="flex min-w-0 items-center gap-2 font-mono text-xs">
						<span className="shrink-0 font-sans text-sm font-medium text-foreground">{hostname(job.url)}</span>
						<span className="truncate" title={job.url}>{job.url}</span>
						<span className="ml-auto shrink-0">{compactId(job.id)}</span>
					</DialogDescription>
				</DialogHeader>

				<div className="relative min-h-0 flex-1 bg-surface-subtle/60">
					<div
						ref={viewport}
						onScroll={handleScroll}
						tabIndex={0}
						aria-label="Crawler log output"
						className="h-full overflow-auto py-2 font-mono text-xs leading-relaxed"
					>
						{lines.length === 0 ? (
							<div className="grid h-full place-items-center text-center font-sans">
								<div>
									<p className="font-medium">{connection === "connecting" ? "Waiting for output…" : "No log output yet"}</p>
									<p className="mt-1 text-sm text-muted-foreground">
										{connection === "ended" ? "This job didn’t leave any logs." : "Lines will appear here as the crawler runs."}
									</p>
								</div>
							</div>
						) : (
							lines.map(line => {
								const style = levelStyles[line.level];
								return (
									<div key={line.id} className={cn("grid grid-cols-[4.5rem_7.5rem_1fr] gap-3 px-4 py-px hover:bg-muted", style.row)}>
										<time className="text-muted-foreground tabular-nums">{line.time}</time>
										<span className={cn("truncate", style.tag)} title={line.tag}>{line.tag}</span>
										<span className="min-w-0 break-words whitespace-pre-wrap">
											<span className={style.message}>{line.message}</span>
											{line.details && <span className="ml-2 text-muted-foreground">{line.details}</span>}
										</span>
									</div>
								);
							})
						)}
					</div>
					{!following && lines.length > 0 && (
						<Button
							size="sm"
							variant="secondary"
							className="absolute right-4 bottom-4 shadow-panel"
							onClick={() => setFollowing(true)}
						>
							<ArrowDown className="size-4" />
							Jump to latest
						</Button>
					)}
				</div>

				<div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1 border-t px-5 py-2.5 text-xs text-muted-foreground">
					<span role="status" className="flex items-center gap-2">
						<span
							aria-hidden
							className={cn(
								"size-2 rounded-full",
								connection === "live" && "animate-pulse bg-success",
								(connection === "connecting" || connection === "reconnecting") && "bg-warning",
								connection === "error" && "bg-destructive",
								connection === "ended" && "bg-muted-foreground/50",
							)}
						/>
						{connectionLabels[connection]}
					</span>
					{notice && <span role="status">{notice}</span>}
					<span className="ml-auto tabular-nums">
						{lines.length.toLocaleString()} {lines.length === 1 ? "line" : "lines"}
						{lines.length >= MAX_LINES && " (most recent only)"}
					</span>
				</div>
			</DialogContent>
		</Dialog>
	);
}
