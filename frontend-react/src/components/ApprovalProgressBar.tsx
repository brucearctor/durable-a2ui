/**
 * CSS-only progress bar for the approval workflow.
 * Renders a 3-dot indicator: Submitted → Review → Decision.
 * Infers stage by scanning message data for title/status patterns.
 * Reuses the same CSS classes as StageProgressBar (.progress-bar, etc.)
 */
import { useMemo } from 'react';
import type { MessageLogEntry } from '../hooks/useWorkflowStream';

type ApprovalStage = 'submitted' | 'under_review' | 'decided';
type Decision = 'approved' | 'rejected' | null;

const STAGES: ApprovalStage[] = ['submitted', 'under_review', 'decided'];

const STAGE_LABELS: Record<ApprovalStage, string> = {
  submitted: 'Submitted',
  under_review: 'Review',
  decided: 'Decision',
};

/** Infer current approval stage + decision from the A2UI message stream. */
function inferApprovalState(messages: MessageLogEntry[]): {
  stage: ApprovalStage;
  decision: Decision;
} {
  let stage: ApprovalStage = 'submitted';
  let decision: Decision = null;

  for (const msg of messages) {
    if (msg.type !== 'updateDataModel') continue;

    const raw = JSON.stringify(msg.data);

    // Check for terminal states first
    if (
      raw.includes('\\u2014 Approved') || raw.includes('— Approved') ||
      raw.toLowerCase().includes('"approved"')
    ) {
      stage = 'decided';
      decision = 'approved';
      continue;
    }
    if (
      raw.includes('\\u2014 Rejected') || raw.includes('— Rejected') ||
      raw.toLowerCase().includes('"rejected"')
    ) {
      stage = 'decided';
      decision = 'rejected';
      continue;
    }

    // Check for review stage
    if (
      raw.includes('\\u2014 Under Review') || raw.includes('— Under Review') ||
      raw.includes('Pending Review') || raw.includes('under_review') ||
      raw.toLowerCase().includes('"under review"')
    ) {
      if (stage !== 'decided') {
        stage = 'under_review';
      }
    }

    // Check for submitted stage (the initial state)
    if (
      raw.includes('\\u2014 Submitted') || raw.includes('— Submitted') ||
      raw.includes('Pending Review')
    ) {
      if (stage === 'submitted') {
        stage = 'submitted'; // already default, but explicit
      }
    }
  }

  return { stage, decision };
}

export function ApprovalProgressBar({ messages }: {
  messages: MessageLogEntry[];
}) {
  const { stage, decision } = useMemo(
    () => inferApprovalState(messages),
    [messages],
  );

  const currentIdx = STAGES.indexOf(stage);
  const isRejected = decision === 'rejected';
  const isApproved = decision === 'approved';
  const allDone = stage === 'decided';

  return (
    <div className={`progress-bar ${isApproved ? 'all-done' : ''} ${isRejected ? 'rejected' : ''}`}>
      <div className="progress-track">
        <div
          className={`progress-fill ${isRejected ? 'fill-rejected' : ''}`}
          style={{ width: `${(currentIdx / (STAGES.length - 1)) * 100}%` }}
        />
      </div>
      <div className="progress-dots">
        {STAGES.map((s, i) => {
          let dotClass: string;
          let dotContent: string;

          if (allDone && isRejected) {
            // Rejected: first two dots completed, last dot red ❌
            if (i < STAGES.length - 1) {
              dotClass = 'completed';
              dotContent = '✓';
            } else {
              dotClass = 'rejected';
              dotContent = '❌';
            }
          } else if (allDone && isApproved) {
            // Approved: all green ✓
            dotClass = 'completed';
            dotContent = '✓';
          } else if (i < currentIdx) {
            dotClass = 'completed';
            dotContent = '✓';
          } else if (i === currentIdx) {
            dotClass = 'active';
            dotContent = '';
          } else {
            dotClass = 'pending';
            dotContent = '';
          }

          return (
            <div key={s} className={`progress-dot-group ${dotClass}`}>
              <div className="progress-dot">{dotContent}</div>
              <span className="progress-label">
                {i === STAGES.length - 1 && allDone && decision
                  ? (isApproved ? 'Approved' : 'Rejected')
                  : STAGE_LABELS[s]}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
