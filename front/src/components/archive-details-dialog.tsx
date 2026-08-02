import type { Archive } from "@/models/archive";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
	DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import {
	Calendar,
	ExternalLink,
	FileText,
	Tag,
	Trash2,
	Pencil,
	Check,
	X,
	AlertCircle,
} from "lucide-react";
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api";
import { queryKeys } from "@/lib/queries";
import { toast } from "sonner";
import { displayArchiveName, formatBytes, formatDateTime } from "@/lib/format";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface Props {
	archive: Archive | null;
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onDeleted: (archiveId: string) => void;
	onUpdated: (updatedArchive: Archive) => void;
}

export function ArchiveDetailsDialog({
	archive,
	open,
	onOpenChange,
	onDeleted,
	onUpdated,
}: Props) {
	const queryClient = useQueryClient();
	const [isEditing, setIsEditing] = useState(false);
	const [editName, setEditName] = useState("");
	const [editDescription, setEditDescription] = useState("");
	const [editTags, setEditTags] = useState("");
	const [isDeleting, setIsDeleting] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);
	const updateArchive = useMutation({
		mutationFn: ({
			id,
			payload,
		}: {
			id: string;
			payload: { name: string; description: string; tags: string[] };
		}) => apiClient.put(`/archives/${id}`, payload),
	});
	const deleteArchive = useMutation({
		mutationFn: (id: string) => apiClient.delete(`/archives/${id}`),
	});
	const isLoading = updateArchive.isPending || deleteArchive.isPending;

	if (!archive) return null;

	const resetEditorForArchive = () => {
		setEditName(displayArchiveName(archive.name));
		setEditDescription(archive.description || "");
		setEditTags(archive.tags?.join(", ") || "");
	};

	const handleOpenChange = (nextOpen: boolean) => {
		if (!nextOpen) {
			setIsEditing(false);
			setIsDeleting(false);
			setError(null);
			setSuccess(null);
		}

		onOpenChange(nextOpen);
	};

	const handleStartEditing = () => {
		resetEditorForArchive();
		setError(null);
		setSuccess(null);
		setIsEditing(true);
	};

	const handleCancelEditing = () => {
		resetEditorForArchive();
		setError(null);
		setIsEditing(false);
	};

	const handleUpdate = async () => {
		if (!editName.trim()) {
			setError("Name cannot be empty");
			return;
		}

		// Parse tags: split on comma, trim, drop empty
		const parsedTags = editTags
			.split(",")
			.map((tag) => tag.trim())
			.filter((tag) => tag.length > 0);

		setError(null);
		setSuccess(null);
		try {
			const payload = {
				name: editName,
				description: editDescription,
				tags: parsedTags,
			};

			await updateArchive.mutateAsync({ id: archive.id, payload });

			const updatedArchive: Archive = {
				...archive,
				name: editName,
				description: editDescription,
				tags: parsedTags,
			};

			// Notify parent about the update
			onUpdated(updatedArchive);
			void queryClient.invalidateQueries({ queryKey: queryKeys.archives });

			setIsEditing(false);
			setSuccess("Archive updated successfully");
			toast.success(`Archive "${editName}" updated`);
		} catch (err: unknown) {
			setError(err instanceof Error ? err.message : "Failed to update archive");
		}
	};

	const handleDelete = async () => {
		setError(null);
		try {
			await deleteArchive.mutateAsync(archive.id);
			void queryClient.invalidateQueries({ queryKey: queryKeys.archives });
			toast.success(`Archive "${displayArchiveName(archive.name)}" deleted`);
			onDeleted(archive.id);
			onOpenChange(false);
			setIsDeleting(false);
		} catch (err: unknown) {
			setError(err instanceof Error ? err.message : "Failed to delete archive");
			setIsDeleting(false);
		}
	};

	const formattedDate = formatDateTime(archive.created_at);

	return (
		<>
			<Dialog open={open} onOpenChange={handleOpenChange}>
				<DialogContent className="sm:max-w-125">
					<DialogHeader>
						<DialogTitle className="flex items-center gap-2">
							<FileText className="size-5 text-primary" />
							Archive Details
						</DialogTitle>
					</DialogHeader>

					<div className="max-h-[65dvh] space-y-6 overflow-y-auto py-4 pr-1">
						{/* Name */}
						<div className="space-y-2">
							<Label className="text-muted-foreground">Name</Label>
							{isEditing ? (
								<Input
									value={editName}
									onChange={(e) => setEditName(e.target.value)}
									className="h-9"
									autoFocus
								/>
							) : (
								<div className="text-lg font-semibold wrap-break-word">
									{displayArchiveName(archive.name)}
								</div>
							)}
						</div>

						{/* File Name */}
						{!isEditing && (
							<div className="space-y-1">
								<Label className="text-muted-foreground">Filename</Label>
								<div className="text-sm font-mono bg-muted p-2 rounded-md break-all">
									{archive.filename}
								</div>
							</div>
						)}

						{/* Description */}
						<div className="space-y-1">
							<Label className="text-muted-foreground">Description</Label>
							{isEditing ? (
								<Textarea
									value={editDescription}
									onChange={(e) => setEditDescription(e.target.value)}
									placeholder="Enter archive description..."
									className="min-h-25 resize-none"
								/>
							) : (
								<div className="text-sm min-h-15 p-2 rounded-md border bg-muted/30 whitespace-pre-wrap">
									{archive.description || (
										<span className="text-muted-foreground italic">
											No description provided
										</span>
									)}
								</div>
							)}
						</div>

						{/* Metadata Grid */}
						{!isEditing && (
							<div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
								<div className="space-y-1">
									<Label className="text-muted-foreground flex items-center gap-1">
										<ExternalLink className="size-3" /> Source URL
									</Label>
									<div className="text-sm break-all">
										{archive.source_url ? (
											<a
												href={archive.source_url}
												target="_blank"
												rel="noopener noreferrer"
												className="text-primary hover:underline"
											>
												{archive.source_url}
											</a>
										) : (
											<span className="text-muted-foreground italic">
												No URL
											</span>
										)}
									</div>
								</div>
								<div className="space-y-1">
									<Label className="text-muted-foreground flex items-center gap-1">
										<Calendar className="size-3" /> Created
									</Label>
									<div className="text-sm">{formattedDate}</div>
								</div>
								<div className="space-y-1">
									<Label className="text-muted-foreground flex items-center gap-1">
										<FileText className="size-3" /> Size
									</Label>
									<div className="text-sm">
										{formatBytes(archive.size_bytes || 0)}
									</div>
								</div>
							</div>
						)}

						{/* Tags */}
						<div className="space-y-2">
							<Label className="text-muted-foreground flex items-center gap-1">
								<Tag className="size-3" /> Tags
							</Label>
							{isEditing ? (
								<Input
									value={editTags}
									onChange={(e) => setEditTags(e.target.value)}
									placeholder="tag1, tag2, tag3"
								/>
							) : (
								<div className="flex flex-wrap gap-1">
									{archive.tags && archive.tags.length > 0 ? (
										archive.tags.map((tag) => (
											<Badge key={tag} variant="secondary">
												{tag}
											</Badge>
										))
									) : (
										<span className="text-sm text-muted-foreground italic">
											No tags
										</span>
									)}
								</div>
							)}
						</div>

						{error && (
							<div className="flex items-center gap-2 text-sm text-destructive bg-destructive/10 p-2 rounded-md">
								<AlertCircle className="size-4" />
								{error}
							</div>
						)}

						{success && (
							<div className="flex items-center gap-2 rounded-md bg-success/15 p-2 text-sm text-foreground">
								<Check className="size-4" />
								{success}
							</div>
						)}
					</div>

					<DialogFooter className="flex-col gap-2 sm:flex-row sm:justify-end">
						{isEditing ? (
							<div className="flex items-center gap-2 w-full">
								<Button
									className="flex-1"
									onClick={handleUpdate}
									disabled={isLoading}
								>
									<Check className="mr-2 size-4" />
									Save Changes
								</Button>
								<Button
									variant="outline"
									className="flex-1"
									onClick={handleCancelEditing}
									disabled={isLoading}
								>
									<X className="mr-2 size-4" />
									Cancel
								</Button>
							</div>
						) : (
							<div className="flex items-center gap-2">
								<Button
									variant="outline"
									size="sm"
									className="h-9"
									onClick={handleStartEditing}
								>
									<Pencil className="mr-2 size-4" />
									Edit Metadata
								</Button>
								<Button
									variant="destructive"
									size="sm"
									className="h-9"
									onClick={() => setIsDeleting(true)}
									disabled={isLoading}
								>
									<Trash2 className="mr-2 size-4" />
									Delete
								</Button>
							</div>
						)}
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<AlertDialog open={isDeleting} onOpenChange={setIsDeleting}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
						<AlertDialogDescription>
							This action cannot be undone. This will permanently delete the
							archive
							<span className="font-semibold text-foreground">
								{" "}
								{archive.name}{" "}
							</span>
							and remove all associated metadata.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={isLoading}>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={(e) => {
								e.preventDefault();
								handleDelete();
							}}
							className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
							disabled={isLoading}
						>
							{isLoading ? "Deleting..." : "Delete Archive"}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</>
	);
}
