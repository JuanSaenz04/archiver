export interface Job {
    id: string;
    url: string;
    status: string;
    created_at: string;
}

export type GetJobsResponse = Job[];