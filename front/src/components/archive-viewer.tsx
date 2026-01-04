interface Props {
    archiveName: string
}

export function ArchiveViewer({ archiveName } : Props) {
  const source = `/api/archives/${archiveName}`
  const viewerUrl = `/viewer.html?source=${encodeURIComponent(source)}`
  
    return (
  
      <div className="h-[80vh] w-full bg-gray-100 rounded-md overflow-hidden border">
  
          {archiveName ? (
  
              <iframe 
  
                  key={viewerUrl}
  
                  src={viewerUrl}
  
                  className="w-full h-full border-none"
  
                  title="Archive Viewer"
  
              />
  
          ) : (
  
              <div className="flex items-center justify-center h-full text-gray-500">
  
                  Select an archive to view
  
              </div>
  
          )}
  
      </div>
  
    )
  
  }
  
  