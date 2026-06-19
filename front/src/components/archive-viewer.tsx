interface Props {
  archiveId: string;
}

export function ArchiveViewer({ archiveId }: Props) {
  const source = `/api/archives/${archiveId}`;
  const viewerUrl = `/viewer.html?source=${encodeURIComponent(source)}`;

  return (
    <div className="h-full w-full bg-muted/50 rounded-xl overflow-hidden border shadow-sm">
      {archiveId ? (
        <iframe
          key={viewerUrl}
          src={viewerUrl}
          className="w-full h-full border-none"
          title="Archive Viewer"
        />
      ) : (
        <div className="flex items-center justify-center h-full text-muted-foreground">
          Select an archive to view
        </div>
      )}
    </div>
  );
}
