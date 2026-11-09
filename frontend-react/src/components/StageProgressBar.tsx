/**
 * CSS-only progress bar for order stages.
 * Renders a connected-dots progress indicator outside the A2UI surface.
 * Tracks stage by scanning message data for stage keywords in titles.
 */
import { useMemo } from 'react';
import type { MessageLogEntry } from '../hooks/useWorkflowStream';

const STAGES = ['placed', 'confirmed', 'preparing', 'shipped', 'delivered'];

const STAGE_LABELS: Record<string, string> = {
  placed: 'Placed',
  confirmed: 'Confirmed',
  preparing: 'Preparing',
  shipped: 'Shipped',
  delivered: 'Delivered',
};

/** Infer current stage from A2UI message stream.
 *  Looks for "Order #XXX — StageName" patterns in updateDataModel values.
 */
function inferStage(messages: MessageLogEntry[]): string {
  let best = 'placed';
  let bestIdx = 0;

  for (const msg of messages) {
    if (msg.type !== 'updateDataModel') continue;

    // Stringify the data and search for stage keywords in title patterns
    const raw = JSON.stringify(msg.data);

    // Match "— Confirmed", "— Preparing", etc. in the title
    for (let j = STAGES.length - 1; j > bestIdx; j--) {
      const stage = STAGES[j];
      const pattern = `\\u2014 ${STAGE_LABELS[stage]}`;  // em-dash from JSON
      const altPattern = `— ${STAGE_LABELS[stage]}`;
      if (raw.includes(pattern) || raw.includes(altPattern)) {
        best = stage;
        bestIdx = j;
        break;
      }
    }
  }

  return best;
}

export function StageProgressBar({ messages, cancelled }: {
  messages: MessageLogEntry[];
  cancelled?: boolean;
}) {
  const currentStage = useMemo(() => inferStage(messages), [messages]);
  const currentIdx = STAGES.indexOf(currentStage);

  if (cancelled) {
    return (
      <div className="progress-bar">
        <div className="progress-cancelled">❌ Cancelled</div>
      </div>
    );
  }

  const allDone = currentStage === 'delivered';

  return (
    <div className={`progress-bar ${allDone ? 'all-done' : ''}`}>
      <div className="progress-track">
        <div
          className="progress-fill"
          style={{ width: `${(currentIdx / (STAGES.length - 1)) * 100}%` }}
        />
      </div>
      <div className="progress-dots">
        {STAGES.map((stage, i) => (
          <div
            key={stage}
            className={`progress-dot-group ${
              allDone || i < currentIdx ? 'completed' :
              i === currentIdx ? 'active' :
              'pending'
            }`}
          >
            <div className="progress-dot">
              {allDone || i < currentIdx ? '✓' : ''}
            </div>
            <span className="progress-label">{STAGE_LABELS[stage]}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
