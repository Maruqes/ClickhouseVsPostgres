import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export default function App() {
  return (
    <main className="flex min-h-svh items-center justify-center bg-background px-6 py-12 text-foreground">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <p className="mb-3 font-mono text-xs uppercase tracking-widest text-muted-foreground">Database comparison</p>
          <CardTitle className="text-2xl">Same application. Two databases.</CardTitle>
          <CardDescription>A shared starting point for comparing ClickHouse and PostgreSQL.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">The foundation is in place. Application features will be added here.</p>
        </CardContent>
      </Card>
    </main>
  )
}
