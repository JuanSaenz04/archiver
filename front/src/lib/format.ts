export const displayArchiveName = (name: string) =>
	name.replace(/\.wacz$/i, "") || "Untitled archive";
export const formatBytes = (bytes?: number) => {
	if (!bytes) return "Size unavailable";
	const units = ["B", "KB", "MB", "GB", "TB"];
	const i = Math.min(
		Math.floor(Math.log(bytes) / Math.log(1024)),
		units.length - 1,
	);
	return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: i ? 1 : 0 }).format(bytes / 1024 ** i)} ${units[i]}`;
};
export const formatDate = (value: string | Date) =>
	new Intl.DateTimeFormat(undefined, {
		year: "numeric",
		month: "short",
		day: "numeric",
	}).format(new Date(value));
export const formatDateTime = (value: string | Date) =>
	new Intl.DateTimeFormat(undefined, {
		year: "numeric",
		month: "short",
		day: "numeric",
		hour: "2-digit",
		minute: "2-digit",
	}).format(new Date(value));
export const hostname = (url?: string) => {
	try {
		return url ? new URL(url).hostname : "Source unavailable";
	} catch {
		return url || "Source unavailable";
	}
};
export const compactId = (id: string) => id.slice(0, 8);
export const dateInputValue = (date: Date) =>
	`${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
