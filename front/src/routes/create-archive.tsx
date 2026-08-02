import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useRef, useState, type FormEvent, type ReactNode } from "react";
import { ChevronDown, Loader2 } from "lucide-react";
import { apiClient } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
export const Route = createFileRoute("/create-archive")({
	component: CreateArchive,
});
const scopes = [
	["page", "Page — this page only"],
	["page-spa", "Page SPA — include hash routes"],
	["prefix", "Prefix — this folder"],
	["host", "Host — this hostname"],
	["domain", "Domain — include subdomains"],
	["any", "Any — follow all links"],
] as const;
function CreateArchive() {
	const navigate = useNavigate();
	const urlRef = useRef<HTMLInputElement>(null);
	const [url, setUrl] = useState("");
	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [tags, setTags] = useState("");
	const [scope, setScope] = useState("page");
	const [depth, setDepth] = useState(2);
	const [pages, setPages] = useState(100);
	const [size, setSize] = useState(0);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const submit = async (e: FormEvent) => {
		e.preventDefault();
		if (!url.trim()) {
			setError("Enter a URL to archive.");
			urlRef.current?.focus();
			return;
		}
		try {
			new URL(url);
		} catch {
			setError("Enter a complete URL, including http:// or https://.");
			urlRef.current?.focus();
			return;
		}
		setLoading(true);
		setError(null);
		try {
			await apiClient.post("/jobs", {
				url,
				name,
				description,
				tags: tags
					.split(",")
					.map((t) => t.trim())
					.filter(Boolean),
				crawl_options: {
					scopeType: scope,
					page_limit: Number(pages),
					size_limit: Number(size),
					depth: Number(depth),
				},
			});
			navigate({ to: "/" });
		} catch (err) {
			setError(
				err instanceof Error ? err.message : "Unable to create archive job.",
			);
		} finally {
			setLoading(false);
		}
	};
	return (
		<div className="min-h-full overflow-y-auto">
			<div className="mx-auto w-full max-w-2xl px-(--content-gutter) py-8 pb-28 md:py-12">
				<header className="mb-8">
					<p className="text-sm font-semibold uppercase tracking-[.16em] text-primary">
						New capture
					</p>
					<h1 className="mt-2 text-3xl font-semibold tracking-tight">
						Archive a website
					</h1>
					<p className="mt-2 text-muted-foreground">
						Start with a target; fine-tune crawl limits only when you need them.
					</p>
				</header>
				<form noValidate onSubmit={submit} className="space-y-7">
					<fieldset className="space-y-4">
						<legend className="mb-3 text-lg font-semibold">Target</legend>
						<Field label="Target URL" id="url" required>
							<Input
								ref={urlRef}
								id="url"
								type="url"
								inputMode="url"
								autoComplete="url"
								placeholder="https://example.com"
								value={url}
								onChange={(e) => setUrl(e.target.value)}
								aria-invalid={!!error && !url}
							/>
						</Field>
						<Field label="Archive name" id="name" optional>
							<Input
								id="name"
								autoComplete="off"
								placeholder="A helpful name for this capture"
								value={name}
								onChange={(e) => setName(e.target.value)}
							/>
						</Field>
					</fieldset>
					<fieldset className="border-t pt-6">
						<legend className="mb-3 text-lg font-semibold">
							Metadata{" "}
							<span className="text-sm font-normal text-muted-foreground">
								optional
							</span>
						</legend>
						<div className="space-y-4">
							<Field label="Description" id="description">
								<Textarea
									id="description"
									value={description}
									onChange={(e) => setDescription(e.target.value)}
									placeholder="What makes this capture useful?"
								/>
							</Field>
							<Field
								label="Tags"
								id="tags"
								description="Separate tags with commas."
							>
								<Input
									id="tags"
									value={tags}
									onChange={(e) => setTags(e.target.value)}
									placeholder="research, launch, reference"
								/>
							</Field>
						</div>
					</fieldset>
					<details className="group border-t pt-6">
						<summary className="flex cursor-pointer list-none items-center justify-between text-lg font-semibold">
							Crawl settings{" "}
							<ChevronDown className="size-5 transition-transform group-open:rotate-180" />
						</summary>
						<p className="mt-2 text-sm text-muted-foreground">
							These limits control how broadly the crawler follows links.
						</p>
						<div className="mt-5 grid gap-4 sm:grid-cols-2">
							<Field
								label="Scope"
								id="scope"
								description="Choose how far links may lead from your target."
							>
								<select
									id="scope"
									value={scope}
									onChange={(e) => setScope(e.target.value)}
									className="h-10 w-full rounded-md border bg-surface px-3 text-sm"
								>
									{scopes.map(([v, l]) => (
										<option key={v} value={v}>
											{l}
										</option>
									))}
								</select>
							</Field>
							<Field
								label="Depth"
								id="depth"
								description="Use -1 for unlimited depth."
							>
								<Input
									id="depth"
									type="number"
									min="-1"
									step="1"
									value={depth}
									onChange={(e) => setDepth(Number(e.target.value))}
								/>
							</Field>
							<Field
								label="Page limit"
								id="pages"
								description="0 means no page limit."
							>
								<Input
									id="pages"
									type="number"
									min="0"
									step="1"
									value={pages}
									onChange={(e) => setPages(Number(e.target.value))}
								/>
							</Field>
							<Field
								label="Size limit (MB)"
								id="size"
								description="0 means no size limit."
							>
								<Input
									id="size"
									type="number"
									min="0"
									step="1"
									value={size}
									onChange={(e) => setSize(Number(e.target.value))}
								/>
							</Field>
						</div>
					</details>
					{error && (
						<p
							role="alert"
							className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
						>
							{error}
						</p>
					)}
					<div className="fixed inset-x-0 bottom-0 z-20 border-t bg-background/95 p-3 backdrop-blur md:static md:border-0 md:bg-transparent md:p-0">
						<div className="mx-auto flex max-w-2xl justify-end">
							<Button
								type="submit"
								disabled={loading}
								className="w-full sm:w-auto"
							>
								{loading && <Loader2 className="size-4 animate-spin" />}
								{loading ? "Starting archive…" : "Start archiving"}
							</Button>
						</div>
					</div>
				</form>
			</div>
		</div>
	);
}
function Field({
	label,
	id,
	children,
	description,
	required,
	optional,
}: {
	label: string;
	id: string;
	children: ReactNode;
	description?: string;
	required?: boolean;
	optional?: boolean;
}) {
	return (
		<div className="space-y-2">
			<Label htmlFor={id}>
				{label}
				{required && <span className="text-destructive"> *</span>}
				{optional && (
					<span className="ml-1 text-muted-foreground">optional</span>
				)}
			</Label>
			{children}
			{description && (
				<p id={`${id}-help`} className="text-xs text-muted-foreground">
					{description}
				</p>
			)}
		</div>
	);
}
