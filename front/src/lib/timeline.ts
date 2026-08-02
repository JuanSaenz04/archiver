import type { Archive } from "@/models/archive";

const MINIMUM_RANGE_PADDING_MS = 60 * 60 * 1000;

/** Adds context around the oldest and newest capture without excluding either. */
export function paddedArchiveRange(archives: Archive[]) {
	const first = archives.at(0);
	const last = archives.at(-1);
	if (!first || !last) return null;

	const firstTime = new Date(first.created_at).getTime();
	const lastTime = new Date(last.created_at).getTime();
	const span = Math.max(lastTime - firstTime, 24 * 60 * 60 * 1000);
	const padding = Math.max(span * 0.04, MINIMUM_RANGE_PADDING_MS);

	return {
		start: new Date(firstTime - padding),
		end: new Date(lastTime + padding),
	};
}
