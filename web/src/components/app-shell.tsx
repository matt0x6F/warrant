import { Link, Outlet } from 'react-router-dom'

import { Button } from '@/components/ui/button'
import { useAuth } from '@/contexts/use-auth'

export function AppShell() {
  const { token, signOut } = useAuth()

  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-border border-b bg-background/80 backdrop-blur">
        <div className="mx-auto flex h-12 max-w-4xl items-center justify-between gap-4 px-4">
          <nav className="flex items-center gap-3 text-sm">
            <Link
              to="/"
              className="text-foreground font-medium tracking-tight hover:underline"
            >
              Warrant
            </Link>
            {token ? (
              <>
                <Link
                  to="/orgs"
                  className="text-muted-foreground hover:text-foreground"
                >
                  Organizations
                </Link>
              </>
            ) : null}
          </nav>
          <div className="flex items-center gap-2">
            {token ? (
              <Button type="button" variant="outline" size="sm" onClick={signOut}>
                Sign out
              </Button>
            ) : (
              <Button asChild size="sm">
                <a href="/auth/github">Sign in with GitHub</a>
              </Button>
            )}
          </div>
        </div>
      </header>
      <main className="mx-auto w-full max-w-4xl flex-1 p-4">
        <Outlet />
      </main>
    </div>
  )
}
