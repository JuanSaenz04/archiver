import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { apiClient } from '@/lib/api'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { ArrowLeft, Loader2, Info } from 'lucide-react'
import { cn } from '@/lib/utils'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

export const Route = createFileRoute('/create-archive')({
  component: CreateArchive,
})

// Scope types from internal/models/options.go
const SCOPE_TYPES = [
  { value: 'page', label: 'Page (Single Page)' },
  { value: 'page-spa', label: 'Page (SPA)' },
  { value: 'prefix', label: 'Prefix' },
  { value: 'host', label: 'Host' },
  { value: 'domain', label: 'Domain' },
  { value: 'any', label: 'Any' },
]

function CreateArchive() {
  const navigate = useNavigate()
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Form State
  const [url, setUrl] = useState('')
  const [name, setName] = useState('')
  const [scopeType, setScopeType] = useState('page')
  const [pageLimit, setPageLimit] = useState(100)
  const [sizeLimit, setSizeLimit] = useState(0) // 0 usually means unlimited or default
  const [depth, setDepth] = useState(2)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError(null)

    if (!url) {
      setError("URL is required")
      setIsLoading(false)
      return
    }

    try {
      const payload = {
        url: url,
        crawl_options: {
          name: name,
          scopeType: scopeType,
          page_limit: Number(pageLimit),
          size_limit: Number(sizeLimit),
          depth: Number(depth),
        }
      }

      await apiClient.post('/jobs', payload)
      
      // Navigate back to home on success
      navigate({ to: '/' })
    } catch (err: any) {
      console.error(err)
      setError(err.message || "Failed to create archive job")
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex flex-col items-center justify-center min-h-screen p-4 bg-muted/20">
      <div className="w-full max-w-lg space-y-6">
        
        <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon" onClick={() => navigate({ to: '/' })}>
                <ArrowLeft className="size-5" />
            </Button>
            <h1 className="text-2xl font-bold tracking-tight">Create New Archive</h1>
        </div>

        <div className="p-6 rounded-xl border bg-card text-card-foreground shadow-sm">
            <form onSubmit={handleSubmit} className="space-y-4">
                
                {/* URL */}
                <div className="space-y-2">
                    <label htmlFor="url" className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                        Target URL
                    </label>
                    <Input 
                        id="url"
                        placeholder="https://example.com"
                        value={url}
                        onChange={(e) => setUrl(e.target.value)}
                        required
                    />
                </div>

                {/* Name */}
                <div className="space-y-2">
                    <label htmlFor="name" className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                        Name (Optional)
                    </label>
                    <Input 
                        id="name"
                        placeholder="My Crawl Job"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                    />
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {/* Scope Type */}
                    <div className="space-y-2">
                        <div className="flex items-center gap-2">
                            <label htmlFor="scopeType" className="text-sm font-medium leading-none">
                                Scope
                            </label>
                            <Tooltip>
                                <TooltipTrigger>
                                    <Info className="size-4 text-muted-foreground" />
                                </TooltipTrigger>
                                <TooltipContent className="max-w-xs space-y-2 p-4">
                                    <p><span className="font-bold">page</span>: crawl only this page and no additional links.</p>
                                    <p><span className="font-bold">page-spa</span>: crawl only this page, but load any links that include different hashtags. Useful for single-page apps that may load different content based on hashtag.</p>
                                    <p><span className="font-bold">prefix</span>: crawl any pages in the same directory, eg. starting from https://example.com/path/page.html, crawl anything under https://example.com/path/ (default)</p>
                                    <p><span className="font-bold">host</span>: crawl pages that share the same host.</p>
                                    <p><span className="font-bold">domain</span>: crawl pages that share the same domain and subdomains, eg. given https://example.com/ will also crawl https://anysubdomain.example.com/</p>
                                    <p><span className="font-bold">any</span>: crawl any and all pages linked from this page.</p>
                                </TooltipContent>
                            </Tooltip>
                        </div>
                        <select
                            id="scopeType"
                            className={cn(
                                "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors outline-none",
                                "focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]",
                                "dark:bg-input/30"
                            )}
                            value={scopeType}
                            onChange={(e) => setScopeType(e.target.value)}
                        >
                            {SCOPE_TYPES.map(type => (
                                <option key={type.value} value={type.value} className="bg-background text-foreground">
                                    {type.label}
                                </option>
                            ))}
                        </select>
                    </div>

                    {/* Depth */}
                    <div className="space-y-2">
                        <label htmlFor="depth" className="text-sm font-medium leading-none">
                            Depth
                        </label>
                        <Input 
                            id="depth"
                            type="number"
                            min="-1"
                            value={depth}
                            onChange={(e) => setDepth(Number(e.target.value))}
                        />
                        <p className="text-[0.8rem] text-muted-foreground">-1 for unlimited</p>
                    </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {/* Page Limit */}
                    <div className="space-y-2">
                        <label htmlFor="pageLimit" className="text-sm font-medium leading-none">
                            Page Limit
                        </label>
                        <Input 
                            id="pageLimit"
                            type="number"
                            min="0"
                            value={pageLimit}
                            onChange={(e) => setPageLimit(Number(e.target.value))}
                        />
                    </div>

                    {/* Size Limit */}
                    <div className="space-y-2">
                         <label htmlFor="sizeLimit" className="text-sm font-medium leading-none">
                            Size Limit (MB)
                        </label>
                        <Input 
                            id="sizeLimit"
                            type="number"
                            min="0"
                            value={sizeLimit}
                            onChange={(e) => setSizeLimit(Number(e.target.value))}
                        />
                        <p className="text-[0.8rem] text-muted-foreground">0 for unlimited</p>
                    </div>
                </div>

                {error && (
                    <div className="text-sm text-destructive font-medium p-2 bg-destructive/10 rounded-md">
                        {error}
                    </div>
                )}

                <div className="pt-4 flex justify-end">
                    <Button type="submit" disabled={isLoading}>
                        {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        Start Archiving
                    </Button>
                </div>

            </form>
        </div>
      </div>
    </div>
  )
}
