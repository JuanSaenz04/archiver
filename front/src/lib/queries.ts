import { queryOptions } from "@tanstack/react-query";
import { apiClient } from "@/lib/api";
import type { GetArchivesResponse } from "@/models/archive";
import type { Job } from "@/models/job";

type JobsResponse = Job[] | { jobs?: Job[] };

export const queryKeys = {
	archives: ["archives"] as const,
	jobs: ["jobs"] as const,
};

export const archivesQueryOptions = queryOptions({
	queryKey: queryKeys.archives,
	queryFn: async () =>
		(await apiClient.get<GetArchivesResponse>("/archives")).archives,
	staleTime: 30_000,
});

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
