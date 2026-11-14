/**
 * Worker kill/restart controls for the Temporal durability demo.
 *
 * Implements a "3-Act Play" for the kill-worker demo:
 *   Act 1 (Setup): Show what's at stake — cart contents, conversation state
 *   Act 2 (Crisis): Worker dies — dramatic overlay with "what you'd lose" contrast
 *   Act 3 (Recovery): Restart → Temporal replays → everything comes back
 */
import { useState, useCallback, useEffect } from 'react';
import {
  Skull, Unplug, RotateCw, ShieldCheck, ShieldX,
  CheckCircle, XCircle, Timer
} from 'lucide-react';

export function useWorkerStatus() {
  const [workerAlive, setWorkerAlive] = useState(true);
  const [killing, setKilling] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const [downSince, setDownSince] = useState<number | null>(null);
  const [a2aOutage, setA2aOutage] = useState(false);
  const [togglingChaos, setTogglingChaos] = useState(false);

  // Poll worker and A2A chaos status
  useEffect(() => {
    const interval = setInterval(async () => {
      try {
        const res = await fetch('/api/admin/worker-status');
        const data = await res.json();
        const wasAlive = workerAlive;
        setWorkerAlive(data.running);
        if (data.running && !wasAlive) {
          setDownSince(null); // recovered
        }
        const chaosRes = await fetch('/api/a2a/chaos');
        if (chaosRes.ok) {
          const chaosData = await chaosRes.json();
          setA2aOutage(Boolean(chaosData.outage));
        }
      } catch {
        // BFF might be down too
      }
    }, 2000);
    return () => clearInterval(interval);
  }, [workerAlive]);

  const toggleA2AChaos = useCallback(async () => {
    setTogglingChaos(true);
    try {
      const next = !a2aOutage;
      const res = await fetch('/api/a2a/chaos', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ outage: next }),
      });
      if (res.ok) {
        const data = await res.json();
        setA2aOutage(Boolean(data.outage));
      }
    } catch (err) {
      console.error('Failed to toggle A2A chaos:', err);
    } finally {
      setTogglingChaos(false);
    }
  }, [a2aOutage]);

  const killWorker = useCallback(async () => {
    setKilling(true);
    try {
      await fetch('/api/admin/kill-worker', { method: 'POST' });
      setWorkerAlive(false);
      setDownSince(Date.now());
    } catch (err) {
      console.error('Failed to kill worker:', err);
    } finally {
      setKilling(false);
    }
  }, []);

  const restartWorker = useCallback(async () => {
    setRestarting(true);
    try {
      await fetch('/api/admin/start-worker', { method: 'POST' });
      // Wait a moment for it to come up
      await new Promise(r => setTimeout(r, 2000));
      setWorkerAlive(true);
      setDownSince(null);
    } catch (err) {
      console.error('Failed to restart worker:', err);
    } finally {
      setRestarting(false);
    }
  }, []);

  return {
    workerAlive,
    killing,
    restarting,
    killWorker,
    restartWorker,
    downSince,
    a2aOutage,
    togglingChaos,
    toggleA2AChaos,
  };
}

export function WorkerKillButton({ onKill, killing, workerAlive }: {
  onKill: () => void;
  killing: boolean;
  workerAlive: boolean;
}) {
  if (!workerAlive) {
    return (
      <span className="worker-dead-badge" style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
        <Skull size={14} /> Worker down
      </span>
    );
  }

  return (
    <button
      className="ctrl-btn danger"
      onClick={onKill}
      disabled={killing}
      aria-label={killing ? 'Killing worker...' : 'Kill the Temporal worker process'}
      style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
    >
      {killing ? <><Skull size={14} /> Killing...</> : <><Unplug size={14} /> Kill Worker</>}
    </button>
  );
}

function DowntimeCounter({ since }: { since: number }) {
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setElapsed(Math.floor((Date.now() - since) / 1000));
    }, 100);
    return () => clearInterval(interval);
  }, [since]);

  const mins = Math.floor(elapsed / 60);
  const secs = elapsed % 60;
  return (
    <span className="downtime-counter">
      <Timer size={14} />
      {mins > 0 ? `${mins}m ${secs}s` : `${secs}s`} down
    </span>
  );
}

export function WorkerDeadOverlay({ onRestart, restarting, downSince }: {
  onRestart: () => void;
  restarting: boolean;
  downSince: number | null;
}) {
  return (
    <div className="worker-dead-overlay">
      <div className="worker-dead-content">
        {/* Act 2: The Crisis */}
        <div className="worker-dead-icon"><Skull size={48} /></div>
        <h2>Worker Process Killed</h2>
        <p>The Temporal Worker is dead. But your workflow is safe.</p>
        {downSince && <DowntimeCounter since={downSince} />}

        {/* Split comparison: Durable vs Fragile */}
        <div className="durability-comparison">
          <div className="comparison-column durable">
            <div className="comparison-header">
              <ShieldCheck size={20} />
              <span>With Temporal</span>
            </div>
            <div className="comparison-items">
              <div className="comparison-item safe">
                <CheckCircle size={14} />
                <span>Workflow state — safe on Temporal Server</span>
              </div>
              <div className="comparison-item safe">
                <CheckCircle size={14} />
                <span>Cart contents — persisted in event history</span>
              </div>
              <div className="comparison-item safe">
                <CheckCircle size={14} />
                <span>Conversation — replayed on restart</span>
              </div>
              <div className="comparison-item safe">
                <CheckCircle size={14} />
                <span>A2UI stream — rebuilt from events</span>
              </div>
            </div>
          </div>

          <div className="comparison-divider">
            <span>vs</span>
          </div>

          <div className="comparison-column fragile">
            <div className="comparison-header">
              <ShieldX size={20} />
              <span>Without Temporal</span>
            </div>
            <div className="comparison-items">
              <div className="comparison-item lost">
                <XCircle size={14} />
                <span>Workflow state — <strong>gone</strong></span>
              </div>
              <div className="comparison-item lost">
                <XCircle size={14} />
                <span>Cart contents — <strong>lost forever</strong></span>
              </div>
              <div className="comparison-item lost">
                <XCircle size={14} />
                <span>Conversation — <strong>vanished</strong></span>
              </div>
              <div className="comparison-item lost">
                <XCircle size={14} />
                <span>User must <strong>start over</strong></span>
              </div>
            </div>
          </div>
        </div>

        {/* Act 3: Recovery */}
        <button
          className="place-order-btn"
          onClick={onRestart}
          disabled={restarting}
          aria-label={restarting ? 'Restarting worker...' : 'Restart worker and watch Temporal recovery'}
        >
          {restarting ? (
            <><RotateCw size={18} className="spin" /> Restarting — Temporal is replaying...</>
          ) : (
            <><RotateCw size={18} /> Restart Worker & Watch Recovery</>
          )}
        </button>
        <p className="worker-dead-footnote">
          When the worker restarts, Temporal replays the entire workflow deterministically.
          <br />Every LLM call, tool call, and agent handoff is rebuilt. No state was lost.
        </p>
      </div>
    </div>
  );
}
