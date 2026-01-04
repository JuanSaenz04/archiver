import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/create-archive')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/create-archive"!</div>
}
