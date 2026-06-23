import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

const capabilities = [
  { name: "Read", active: true },
  { name: "Write", active: false },
  { name: "Groups", active: false },
  { name: "Own password", active: true },
  { name: "Reset", active: false },
]

export default function App() {
  return (
    <main className="min-h-svh bg-background text-foreground">
      <div className="mx-auto flex min-h-svh w-full max-w-6xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
        <header className="flex flex-col gap-4 border-b pb-5 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-col gap-1">
            <p className="font-mono text-xs uppercase tracking-normal text-muted-foreground">
              dc=example,dc=com
            </p>
            <h1 className="text-2xl font-semibold tracking-normal">LDAPLite directory console</h1>
            <p className="max-w-2xl text-sm text-muted-foreground">
              Embedded React and shadcn/ui foundation for the role-aware admin surface.
            </p>
          </div>
          <Button>Open directory</Button>
        </header>

        <section className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_18rem]">
          <Card>
            <CardHeader>
              <CardTitle>Directory workbench</CardTitle>
              <CardDescription>
                The next milestone wires these surfaces to server-side capabilities and APIs.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-3 sm:grid-cols-3">
                {["Users", "Groups", "Organizational units"].map((label) => (
                  <div key={label} className="rounded-md border bg-card p-4">
                    <div className="text-sm font-medium">{label}</div>
                    <div className="mt-1 text-sm text-muted-foreground">Ready for API binding</div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Capability rail</CardTitle>
              <CardDescription>Server-derived permissions will drive navigation and actions.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-col gap-2">
                {capabilities.map((capability) => (
                  <div key={capability.name} className="flex items-center justify-between gap-3">
                    <span className="text-sm">{capability.name}</span>
                    <Badge variant={capability.active ? "default" : "secondary"}>
                      {capability.active ? "Allowed" : "Denied"}
                    </Badge>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </section>
      </div>
    </main>
  )
}
