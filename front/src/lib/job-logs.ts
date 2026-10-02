export interface LogEntry { id: string; time: string; source: string; text: string }

export function watchJobLogs(jobID: string, callbacks: {
	onLines: (lines: LogEntry[]) => void;
	onStatus: (status: string) => void;
	onNotice: (notice: string) => void;
	onDone: () => void;
}) {
	const source = new EventSource(`/api/jobs/${encodeURIComponent(jobID)}/logs`);
	let pending: LogEntry[] = [];
	let lastID = "";
	let closed = false;
	const flush = () => {
		if (!pending.length) return;
		const batch = pending;
		pending = [];
		callbacks.onLines(batch);
	};
	const timer = window.setInterval(flush, 200);
	const close = () => {
		closed = true;
		source.close();
		window.clearInterval(timer);
		pending = [];
	};
	source.addEventListener("open", () => { if (!closed) callbacks.onStatus("Connected"); });
	source.addEventListener("status", event => { if (!closed) callbacks.onStatus(JSON.parse(event.data as string) as string); });
	source.addEventListener("gap", event => { if (!closed) callbacks.onNotice(JSON.parse(event.data as string) as string); });
	source.addEventListener("logs", event => {
		if (closed) return;
		const entries = JSON.parse(event.data as string) as LogEntry[];
		for (const entry of entries) {
			if (lastID && compareIDs(entry.id, lastID) <= 0) continue;
			pending.push(entry);
			lastID = entry.id;
		}
		pending = pending.slice(-2000);
	});
	source.addEventListener("done", event => {
		if (closed) return;
		const result = JSON.parse(event.data as string) as { status: string; unavailable: boolean };
		flush();
		callbacks.onStatus(result.unavailable ? `${result.status} · Logs expired or unavailable` : result.status);
		close();
		callbacks.onDone();
	});
	source.onerror = () => {
		if (!closed) callbacks.onStatus(source.readyState === EventSource.CLOSED ? "Unable to open logs. Close and reopen to retry." : "Connection interrupted. Reconnecting…");
	};
	return close;
}

function compareIDs(a: string, b: string) {
	const left = a.split("-").map(BigInt);
	const right = b.split("-").map(BigInt);
	for (let i = 0; i < 2; i++) {
		if (left[i] < right[i]) return -1;
		if (left[i] > right[i]) return 1;
	}
	return 0;
}
