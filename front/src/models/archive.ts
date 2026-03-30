export interface Archive {
    id: string;
    name: string;
    description: string;
    source_url: string;
    tags: string[];
    created_at: string;
}

export interface GetArchivesResponse {
    archives: Archive[]
}