export interface Archive {
    id: string;
    name: string;
    filename: string;
    description: string;
    source_url: string;
    tags: string[];
    created_at: string;
    size_bytes: number;
}

export interface GetArchivesResponse {
	archives: Archive[];
	next_cursor: string;
}

export interface GetArchiveTagsResponse {
	tags: string[];
}
