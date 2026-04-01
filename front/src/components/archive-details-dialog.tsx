import type { Archive } from "@/models/archive"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Label } from "@/components/ui/label"
import { 
  Calendar, 
  ExternalLink, 
  FileText, 
  Tag, 
  Trash2, 
  Pencil, 
  Check, 
  X,
  AlertCircle
} from "lucide-react"
import { useState, useEffect } from "react"
import { apiClient } from "@/lib/api"
import { toast } from "sonner"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"

interface Props {
  archive: Archive | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onDeleted: (archiveId: string) => void
  onUpdated: (updatedArchive: Archive) => void
}

export function ArchiveDetailsDialog({ archive, open, onOpenChange, onDeleted, onUpdated }: Props) {
  const [isEditing, setIsEditing] = useState(false)
  const [editName, setEditName] = useState("")
  const [editDescription, setEditDescription] = useState("")
  const [editTags, setEditTags] = useState("")
  const [isDeleting, setIsDeleting] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  useEffect(() => {
    if (archive) {
      setEditName(archive.name.replace(".wacz", ""))
      setEditDescription(archive.description || "")
      setEditTags(archive.tags?.join(", ") || "")
      setIsEditing(false)
      setError(null)
      // Only clear success if we are opening the dialog or switching to a DIFFERENT archive
      // We check if success was already set to avoid clearing it immediately after rename
    }
  }, [archive, open])

  // Separate effect to clear messages when opening/closing
  useEffect(() => {
    if (open) {
      setError(null)
      setSuccess(null)
    }
  }, [open])

  if (!archive) return null

  const handleUpdate = async () => {
    if (!editName.trim()) {
      setError("Name cannot be empty")
      return
    }

    // Parse tags: split on comma, trim, drop empty
    const parsedTags = editTags
      .split(',')
      .map(tag => tag.trim())
      .filter(tag => tag.length > 0)

    setIsLoading(true)
    setError(null)
    setSuccess(null)
    try {
      const payload = {
        name: editName,
        description: editDescription,
        tags: parsedTags
      }

      await apiClient.put(`/archives/${archive.name}`, payload)
      const newName = editName + ".wacz"
      
      const updatedArchive: Archive = {
        ...archive,
        name: newName,
        description: editDescription,
        tags: parsedTags
      }

      // Notify parent about the update
      onUpdated(updatedArchive)
      
      setIsEditing(false)
      setSuccess("Archive updated successfully")
      toast.success(`Archive "${editName}" updated`)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to update archive")
    } finally {
      setIsLoading(false)
    }
  }

  const handleDelete = async () => {
    setIsLoading(true)
    setError(null)
    try {
      await apiClient.delete(`/archives/${archive.name}`)
      toast.success(`Archive "${archive.name.replace(".wacz", "")}" deleted`)
      onDeleted(archive.id)
      onOpenChange(false)
      setIsDeleting(false)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to delete archive")
      setIsDeleting(false)
    } finally {
      setIsLoading(false)
    }
  }

  const formattedDate = new Date(archive.created_at).toLocaleString()

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return <span className="text-muted-foreground italic">Size not available</span>
    const k = 1024
    const sizes = ['Bytes', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    // Don't go higher than GB
    const sizeIndex = Math.min(i, sizes.length - 1)
    return `${parseFloat((bytes / Math.pow(k, sizeIndex)).toFixed(2))} ${sizes[sizeIndex]}`
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FileText className="size-5 text-primary" />
              Archive Details
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-6 py-4">
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
                <div className="text-lg font-semibold truncate">{archive.name.replace(".wacz", "")}</div>
              )}
            </div>

            {/* File Name */}
            {!isEditing && (
              <div className="space-y-1">
                <Label className="text-muted-foreground">Filename</Label>
                <div className="text-sm font-mono bg-muted p-2 rounded-md break-all">
                  {archive.name}
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
                  className="min-h-[100px] resize-none"
                />
              ) : (
                <div className="text-sm min-h-[60px] p-2 rounded-md border bg-muted/30 whitespace-pre-wrap">
                  {archive.description || <span className="text-muted-foreground italic">No description provided</span>}
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
                  <div className="text-sm truncate">
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
                      <span className="text-muted-foreground italic">No URL</span>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  <Label className="text-muted-foreground flex items-center gap-1">
                    <Calendar className="size-3" /> Created
                  </Label>
                  <div className="text-sm">
                    {formattedDate}
                  </div>
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
                    archive.tags.map(tag => (
                      <Badge key={tag} variant="secondary">{tag}</Badge>
                    ))
                  ) : (
                    <span className="text-sm text-muted-foreground italic">No tags</span>
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
              <div className="flex items-center gap-2 text-sm text-green-600 bg-green-500/10 p-2 rounded-md">
                <Check className="size-4" />
                {success}
              </div>
            )}
          </div>

          <DialogFooter className="flex-row sm:justify-end gap-2">
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
                  onClick={() => setIsEditing(false)}
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
                  onClick={() => setIsEditing(true)}
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
              This action cannot be undone. This will permanently delete the archive
              <span className="font-semibold text-foreground"> {archive.name} </span>
              and remove all associated metadata.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isLoading}>Cancel</AlertDialogCancel>
            <AlertDialogAction 
              onClick={(e) => {
                e.preventDefault()
                handleDelete()
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
  )
}
