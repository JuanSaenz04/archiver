import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { List, RefreshCw } from "lucide-react";
import { compactId, formatDateTime, hostname } from "@/lib/format";
import { cn } from "@/lib/utils";
import { jobsQueryOptions } from "@/lib/queries";
import { StatusPill } from "@/components/status-pill";
import { Button } from "@/components/ui/button";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTitle,
	SheetTrigger,
} from "@/components/ui/sheet";
interface JobsSheetProps {
	open?: boolean;
	onOpenChange?: (open: boolean) => void;
	showTrigger?: boolean;
}

export function JobsSheet({
	open,
	onOpenChange,
	showTrigger = true,
}: JobsSheetProps) {
	const [internalOpen, setInternalOpen] = useState(false);
	const change = (next: boolean) => {
		if (open === undefined) setInternalOpen(next);
		onOpenChange?.(next);
	};
	const isOpen = open ?? internalOpen;
	const {
		data: jobs = [],
		error,
		isFetched,
		isFetching,
		refetch,
	} = useQuery({ ...jobsQueryOptions, enabled: isOpen });
	return (
		<Sheet open={isOpen} onOpenChange={change}>
			{showTrigger && (
				<SheetTrigger asChild>
					<Button size="icon" variant="ghost" aria-label="View crawl jobs">
						<List className="size-4" />
					</Button>
				</SheetTrigger>
			)}
			<SheetContent className="w-[min(28rem,calc(100vw-1rem))] gap-0 p-0">
				<SheetHeader className="shrink-0 border-b pr-12">
					<SheetTitle>Capture jobs</SheetTitle>
					<SheetDescription>
						Monitor queued and completed archive requests.
					</SheetDescription>
				</SheetHeader>
				<div className="flex min-h-0 flex-1 flex-col overflow-y-auto p-4">
					<div className="mb-3 flex justify-end">
						<Button
							variant="outline"
							size="sm"
							onClick={() => void refetch()}
							disabled={isFetching}
						>
							<RefreshCw className={cn("size-4", isFetching && "animate-spin")} />
							Refresh
						</Button>
					</div>
					{isFetching && !isFetched ? (
						<div className="space-y-2">
							{[1, 2, 3].map((i) => (
								<div
									key={i}
									className="h-20 animate-pulse rounded-md bg-muted"
								/>
							))}
						</div>
					) : error ? (
						<div className="grid flex-1 place-items-center text-center">
							<div>
								<p className="font-medium">Couldn’t load jobs</p>
								<p className="my-2 text-sm text-muted-foreground">{error.message}</p>
								<Button size="sm" onClick={() => void refetch()}>
									Try again
								</Button>
							</div>
						</div>
					) : !jobs.length ? (
						<div className="grid flex-1 place-items-center text-center">
							<div>
								<p className="font-medium">No jobs yet</p>
								<p className="mt-1 text-sm text-muted-foreground">
									New capture requests will appear here.
								</p>
							</div>
						</div>
					) : (
						<div className="space-y-2" aria-live="polite">
							{jobs.map((j) => (
								<article
									key={j.id}
									className="rounded-md border bg-surface p-3"
								>
									<div className="flex items-start justify-between gap-2">
										<div className="min-w-0">
											<p className="truncate font-medium">{hostname(j.url)}</p>
											<p
												className="truncate font-mono text-xs text-muted-foreground"
												title={j.url}
											>
												{j.url}
											</p>
										</div>
										<StatusPill status={j.status} />
									</div>
									<div className="mt-3 flex justify-between font-mono text-[.68rem] text-muted-foreground">
										<span>{compactId(j.id)}</span>
										<time>{formatDateTime(j.created_at)}</time>
									</div>
								</article>
							))}
						</div>
					)}
				</div>
			</SheetContent>
		</Sheet>
	);
}
