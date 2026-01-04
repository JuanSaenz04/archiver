interface Props {
    archiveName: string
}

export function ArchiveViewer({ archiveName } : Props) {
  const source = `/api/archives/${archiveName}`
  const viewerUrl = `/viewer.html?source=${encodeURIComponent(source)}`
  
    return (
  
      <div className="h-full w-full bg-muted/50 rounded-xl overflow-hidden border shadow-sm">
  
          {archiveName ? (
  
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
  
    )
  
  }
  
  