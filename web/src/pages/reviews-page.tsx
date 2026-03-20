import { useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'

import { OrgProjectCrumbs } from '@/components/org-project-crumbs'
import { ReviewQueueCelebration } from '@/components/review-queue-celebration'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { WarrantClient } from '@/contexts/auth-context'
import { useAuth } from '@/contexts/use-auth'
import { useProjectBreadcrumbLabel } from '@/hooks/use-project-breadcrumb-label'
import { formatApiError } from '@/lib/api/client'
import type { components } from '@/lib/api/v1'

type Ticket = components['schemas']['Ticket']

function workStreamKey(ticket: Ticket): string {
  return ticket.work_stream_id ?? ''
}

async function loadPendingReviews(
  client: WarrantClient,
  projectId: string,
): Promise<
  { ok: true; tickets: Ticket[] } | { ok: false; message: string }
> {
  const { data, error, response } = await client.GET(
    '/projects/{projectID}/reviews',
    { params: { path: { projectID: projectId } } },
  )
  if (!response.ok) {
    return { ok: false, message: formatApiError(error) }
  }
  return { ok: true, tickets: data?.tickets ?? [] }
}

export function ReviewsPage() {
  const { orgId, projectId } = useParams<{ orgId: string; projectId: string }>()
  const { client } = useAuth()
  const [tickets, setTickets] = useState<Ticket[] | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [notesById, setNotesById] = useState<Record<string, string>>({})
  const [busyId, setBusyId] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [postReviewNext, setPostReviewNext] = useState(false)
  const [postReviewDecision, setPostReviewDecision] = useState<
    'approved' | 'rejected'
  >('approved')
  const [nextReviewTicketId, setNextReviewTicketId] = useState<string | null>(
    null,
  )
  const [justEmptiedQueue, setJustEmptiedQueue] = useState(false)
  const nextStreamReviewRef = useRef<HTMLLIElement | null>(null)
  const projectLabel = useProjectBreadcrumbLabel(projectId)

  useEffect(() => {
    if (!projectId) return
    let cancelled = false
    void (async () => {
      const r = await loadPendingReviews(client, projectId)
      if (cancelled) return
      if (!r.ok) {
        setErr(r.message)
        setTickets([])
        return
      }
      setErr(null)
      setTickets(r.tickets)
    })()
    return () => {
      cancelled = true
    }
  }, [client, projectId])

  async function submitReview(ticketId: string, decision: 'approved' | 'rejected') {
    if (!projectId || tickets === null) return
    const countBefore = tickets.length
    const reviewedTicket = tickets.find((t) => t.id === ticketId)
    const streamKey = reviewedTicket ? workStreamKey(reviewedTicket) : ''
    setBusyId(ticketId)
    setMessage(null)
    setPostReviewNext(false)
    setNextReviewTicketId(null)
    setJustEmptiedQueue(false)
    const { error, response } = await client.POST('/tickets/{ticketID}/reviews', {
      params: { path: { ticketID: ticketId } },
      body: {
        decision,
        notes: notesById[ticketId]?.trim() || undefined,
      },
    })
    setBusyId(null)
    if (!response.ok) {
      setMessage(formatApiError(error))
      return
    }
    if (decision === 'rejected') {
      setMessage('Rejected.')
    } else {
      setMessage(null)
    }
    setNotesById((prev) => {
      const next = { ...prev }
      delete next[ticketId]
      return next
    })
    const r = await loadPendingReviews(client, projectId)
    if (!r.ok) {
      setMessage(r.message)
      return
    }
    setTickets(r.tickets)

    if (r.tickets.length === 0 && countBefore > 0) {
      setMessage(null)
      setJustEmptiedQueue(true)
      setPostReviewNext(false)
      setNextReviewTicketId(null)
      return
    }
    if (r.tickets.length > 0) {
      const nextInStream = r.tickets.find((t) => workStreamKey(t) === streamKey)
      if (nextInStream?.id) {
        setMessage(null)
        setPostReviewDecision(decision)
        setPostReviewNext(true)
        setNextReviewTicketId(nextInStream.id)
        return
      }
    }
    setPostReviewNext(false)
    setNextReviewTicketId(null)
  }

  function goToNextReview() {
    const li = nextStreamReviewRef.current
    li?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    const ta = li?.querySelector('textarea')
    setPostReviewNext(false)
    setNextReviewTicketId(null)
    requestAnimationFrame(() => {
      if (ta instanceof HTMLTextAreaElement) {
        ta.focus()
      }
    })
  }

  if (!orgId || !projectId) {
    return <p className="text-destructive text-sm">Missing route params.</p>
  }
  if (err) {
    return <p className="text-destructive text-sm">{err}</p>
  }
  if (!tickets) {
    return <p className="text-muted-foreground text-sm">Loading…</p>
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <p className="text-muted-foreground text-xs">
          <OrgProjectCrumbs
            orgId={orgId}
            projectId={projectId}
            projectLabel={projectLabel}
          />
          <span className="px-1">/</span>
          <span className="text-foreground" aria-current="page">
            Pending reviews
          </span>
        </p>
        <h1 className="text-xl font-semibold tracking-tight">Pending reviews</h1>
      </div>

      {message ? (
        <p
          className={
            message.startsWith('Rejected')
              ? 'text-muted-foreground text-sm'
              : 'text-destructive text-sm'
          }
        >
          {message}
        </p>
      ) : null}

      {postReviewNext ? (
        <div
          className="border-border bg-muted/50 flex flex-wrap items-center gap-3 rounded-lg border px-4 py-3"
          role="status"
        >
          <span className="text-sm font-medium">
            {postReviewDecision === 'approved' ? 'Approved.' : 'Rejected.'}
          </span>
          <Button
            type="button"
            size="sm"
            onClick={goToNextReview}
            aria-label="Scroll to the next pending review in this work stream"
          >
            Next in this stream
          </Button>
        </div>
      ) : null}

      <ul className="flex flex-col gap-4">
        {tickets.map((t) => (
          <li
            key={t.id}
            ref={
              postReviewNext && t.id === nextReviewTicketId
                ? nextStreamReviewRef
                : undefined
            }
            className="scroll-mt-4"
          >
            <Card>
              <CardHeader>
                <div className="flex flex-wrap items-center gap-2">
                  <CardTitle className="text-base">{t.title ?? t.id}</CardTitle>
                  {t.state ? <Badge variant="secondary">{t.state}</Badge> : null}
                </div>
                <Link
                  to={`/orgs/${orgId}/projects/${projectId}/tickets/${t.id}`}
                  className="text-primary text-xs hover:underline"
                >
                  Open ticket detail
                </Link>
              </CardHeader>
              <CardContent className="flex flex-col gap-3">
                <label className="flex flex-col gap-1 text-sm">
                  <span className="text-muted-foreground">Notes (optional)</span>
                  <textarea
                    className="border-input bg-background min-h-[72px] rounded-md border px-3 py-2 text-sm"
                    value={notesById[t.id ?? ''] ?? ''}
                    onChange={(e) =>
                      setNotesById((prev) => ({
                        ...prev,
                        [t.id ?? '']: e.target.value,
                      }))
                    }
                    rows={3}
                  />
                </label>
                <div className="flex flex-wrap gap-2">
                  <Button
                    size="sm"
                    disabled={busyId === t.id}
                    onClick={() => t.id && submitReview(t.id, 'approved')}
                  >
                    Approve
                  </Button>
                  <Button
                    size="sm"
                    variant="destructive"
                    disabled={busyId === t.id}
                    onClick={() => t.id && submitReview(t.id, 'rejected')}
                  >
                    Reject
                  </Button>
                </div>
              </CardContent>
            </Card>
          </li>
        ))}
      </ul>

      {tickets.length === 0 && justEmptiedQueue ? (
        <ReviewQueueCelebration onDismiss={() => setJustEmptiedQueue(false)} />
      ) : null}

      {tickets.length === 0 && !justEmptiedQueue ? (
        <p className="text-muted-foreground text-sm">No tickets awaiting review.</p>
      ) : null}
    </div>
  )
}
