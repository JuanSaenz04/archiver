import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import { apiClient } from "@/lib/api";
import type {
	GetArchivesResponse,
	GetArchiveTagsResponse,
} from "@/models/archive";
import type { Job } from "@/models/job";

type JobsResponse = Job[] | { jobs?: Job[] };

export const queryKeys = {
	archives: ["archives"] as const,
	archiveTags: ["archives", "tags"] as const,
	jobs: ["jobs"] as const,
};

interface ArchiveFilters {
	search: string;
	tags: string[];
}

export const archivePagesQueryOptions = (filters: ArchiveFilters) =>
	infiniteQueryOptions({
		queryKey: [...queryKeys.archives, "pages", filters] as const,
		queryFn: ({ pageParam }) => {
			const params = new URLSearchParams({ limit: "30" });
			if (pageParam) params.set("cursor", pageParam);
			if (filters.search.trim()) params.set("q", filters.search.trim());
			filters.tags.forEach((tag) => params.append("tag", tag));
			return apiClient.get<GetArchivesResponse>(`/archives?${params}`);
		},
		initialPageParam: "",
		getNextPageParam: (page) => page.next_cursor || undefined,
		staleTime: 30_000,
	});

export const archiveTagsQueryOptions = queryOptions({
	queryKey: queryKeys.archiveTags,
	queryFn: async () =>
		(await apiClient.get<GetArchiveTagsResponse>("/archives/tags")).tags,
	staleTime: 30_000,
});

export const timelineArchivesQueryOptions = (from: Date, to: Date) => {
	const params = new URLSearchParams({
		from: from.toISOString(),
		to: new Date(to.getTime() + 1).toISOString(),
	});
	return queryOptions({
		queryKey: [...queryKeys.archives, "timeline", params.toString()] as const,
		queryFn: async () =>
			(await apiClient.get<GetArchivesResponse>(`/archives?${params}`)).archives,
		staleTime: 30_000,
	});
};

export const jobsQueryOptions = queryOptions({
	queryKey: queryKeys.jobs,
	queryFn: async () => {
		const response = await apiClient.get<JobsResponse>("/jobs");
		const jobs = Array.isArray(response) ? response : (response.jobs ?? []);
		return [...jobs].sort(
			(a, b) =>
				new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
		);
	},
	staleTime: 10_000,
});
