export interface Job {
    id: string;
    url: string;
    status: string;
}

export type GetJobsResponse = Job[];