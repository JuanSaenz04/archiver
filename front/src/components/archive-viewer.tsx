import { useRuntimeConfig } from "@/lib/runtime-config";

interface Props {
  archiveId: string;
}

export function ArchiveViewer({ archiveId }: Props) {
  const { replay_origin: replayOrigin } = useRuntimeConfig();
  const source = `/archives/${archiveId}`;
  const viewerUrl = new URL("/viewer.html", replayOrigin);
  viewerUrl.searchParams.set("source", source);
  const viewerUrlString = viewerUrl.toString();

  return (
    <div className="h-full w-full bg-muted/50 rounded-xl overflow-hidden border shadow-sm">
      {archiveId ? (
        <iframe
          key={viewerUrlString}
          src={viewerUrlString}
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
