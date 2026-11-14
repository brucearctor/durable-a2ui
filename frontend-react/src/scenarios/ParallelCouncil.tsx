import { useState, useCallback, useEffect, useMemo } from 'react';
import { A2uiSurface } from '@a2ui/react';
import {
  Layers,
  Rocket,
  CheckCircle,
  XCircle,
  RefreshCw,
  Loader,
  Activity,
  ShieldCheck,
  Coins,
  ArrowUpCircle,
} from 'lucide-react';
import { useWorkflowStream } from '../hooks/useWorkflowStream';
import type { StreamOptions } from '../hooks/useWorkflowStream';
import { StatusBadge, ConnectionBanner, QRShare } from '../components/shared';
import { useWorkerStatus, WorkerKillButton, WorkerDeadOverlay } from '../components/WorkerStatus';
import { A2AChaosToggle, ProtocolXRayStrip } from '../components/ProtocolXRay';
import DevToolsPanel from '../components/DevToolsPanel';

const COUNCIL_PRESETS = {
  checkout_canary: {
    architect: 'Priya Patel (Principal Systems Architect)',
    releaseName: 'checkout-api v2.14.0-canary',
    environment: 'us-central1 & eu-west1 (Multi-Region GKE)',
  },
  llm_router: {
    architect: 'Marcus Vance (AI Platform Lead)',
    releaseName: 'llm-gateway v3.2.0 (L4 GPU Speculative Decoding)',
    environment: 'us-east1 & asia-northeast1 (GKE L4 Pool)',
  },
};

export default function ParallelCouncil() {
  const [workflowId, setWorkflowId] = useState<string | null>(null);
  const [devMode, setDevMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [presetKey, setPresetKey] = useState<'checkout_canary' | 'llm_router'>('checkout_canary');
  const [architect, setArchitect] = useState(COUNCIL_PRESETS.checkout_canary.architect);
  const [releaseName, setReleaseName] = useState(COUNCIL_PRESETS.checkout_canary.releaseName);
  const [environment, setEnvironment] = useState(COUNCIL_PRESETS.checkout_canary.environment);
  const [initialSchema, setInitialSchema] = useState<number>(1);
  const [currentSchema, setCurrentSchema] = useState<number>(1);
  const [error, setError] = useState<string | null>(null);

  const applyPreset = (key: 'checkout_canary' | 'llm_router') => {
    setPresetKey(key);
    const p = COUNCIL_PRESETS[key];
    setArchitect(p.architect);
    setReleaseName(p.releaseName);
    setEnvironment(p.environment);
  };

  const triggerUpgrade = useCallback(
    async (targetVersion: number) => {
      if (!workflowId) return;
      try {
        await fetch(`/api/council/${workflowId}/upgrade`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ target_version: targetVersion }),
        });
        setCurrentSchema(targetVersion);
      } catch (err) {
        console.error('Schema upgrade failed:', err);
        setError('Failed to send schema upgrade signal.');
      }
    },
    [workflowId]
  );

  const triggerDecision = useCallback(
    async (decision: 'approved' | 'rejected') => {
      if (!workflowId) return;
      try {
        await fetch(`/api/council/${workflowId}/decide`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ decision }),
        });
      } catch (err) {
        console.error('Decision failed:', err);
        setError('Failed to send council decision.');
      }
    },
    [workflowId]
  );

  const streamOptions: StreamOptions | undefined = useMemo(() => {
    if (!workflowId) return undefined;
    return {
      streamUrl: `/api/council/${workflowId}/stream`,
      signalUrl: `/api/council/${workflowId}/signal`,
      onAction: (eventName: string, action?: any) => {
        if (!workflowId) return false;
        if (eventName === 'upgrade_v2' || eventName === 'upgrade_schema') {
          const target = Number(action?.context?.target || 2);
          triggerUpgrade(target);
          return true;
        }
        if (eventName === 'downgrade_v1') {
          triggerUpgrade(1);
          return true;
        }
        if (eventName === 'council_decision') {
          const dec = action?.context?.decision === 'changes_requested' ? 'rejected' : 'approved';
          triggerDecision(dec);
          return true;
        }
        if (eventName === 'approve_rollout') {
          triggerDecision('approved');
          return true;
        }
        if (eventName === 'reject_rollout') {
          triggerDecision('rejected');
          return true;
        }
        fetch(`/api/council/${workflowId}/signal/user_action`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ event: eventName }),
        }).catch((err) => console.error('Failed to send council action:', err));
        return true;
      },
    };
  }, [workflowId, triggerUpgrade, triggerDecision]);

  const { surfaces, status, messages } = useWorkflowStream(workflowId, streamOptions);
  const {
    workerAlive,
    killing,
    restarting,
    killWorker,
    restartWorker,
    downSince,
    a2aOutage,
    togglingChaos,
    toggleA2AChaos,
  } = useWorkerStatus();

  const completedSpecialists = useMemo(() => {
    const done = new Set<string>();
    for (const m of messages) {
      const raw = m.data as any;
      const trace = raw?.updateDataModel?.value;
      if (trace?.type === 'a2a_specialist_done' && trace?.specialist) {
        done.add(String(trace.specialist));
      }
    }
    return done;
  }, [messages]);

  useEffect(() => {
    for (const m of messages) {
      const raw = m.data as any;
      const trace = raw?.updateDataModel?.value;
      if (trace?.type === 'schema_migration' && trace?.version) {
        setCurrentSchema(Number(trace.version));
      }
    }
  }, [messages]);

  const startCouncil = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/council', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          architect,
          release_name: releaseName,
          environment,
          schema_version: initialSchema,
        }),
      });
      const result = await res.json();
      setCurrentSchema(initialSchema);
      setWorkflowId(result.workflow_id);
    } catch (err) {
      console.error('Failed to start parallel council:', err);
      setError('Could not launch Parallel Council. Is the Go backend running?');
    } finally {
      setLoading(false);
    }
  }, [architect, releaseName, environment, initialSchema]);

  const resetDemo = useCallback(() => {
    setWorkflowId(null);
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;

      switch (e.key.toLowerCase()) {
        case 'u':
          triggerUpgrade(currentSchema >= 2 ? 1 : 2);
          break;
        case 'a':
          triggerDecision('approved');
          break;
        case 'x':
          triggerDecision('rejected');
          break;
        case 'n':
          resetDemo();
          break;
        case 'd':
          setDevMode((prev) => !prev);
          break;
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [triggerUpgrade, triggerDecision, resetDemo, currentSchema]);

  if (!workflowId) {
    return (
      <div className="start-section">
        <div className="chat-start">
          <div className="chat-start-icon">
            <Layers size={48} />
          </div>
          <div className="chat-start-title">Parallel A2A Council & Live Schema Upgrade</div>
          <div className="chat-start-subtitle">
            Option A: Concurrent workflow.Go A2A Fan-Out (:9003 + :9004 + :9001) &bull; Option C: Live v1 &rarr; v2 A2UI Schema Migration
          </div>
        </div>

        <div className="order-form">
          <h2>
            <Layers size={20} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '8px' }} />
            Launch Release Readiness Council
          </h2>

          <div className="mode-toggle" style={{ marginBottom: '10px' }}>
            <button
              type="button"
              className={`mode-btn ${presetKey === 'checkout_canary' ? 'active durable' : ''}`}
              onClick={() => applyPreset('checkout_canary')}
            >
              🚀 checkout-api v2.14.0 Canary
            </button>
            <button
              type="button"
              className={`mode-btn ${presetKey === 'llm_router' ? 'active durable' : ''}`}
              onClick={() => applyPreset('llm_router')}
            >
              🧠 llm-gateway v3.2.0 GPU Pool
            </button>
          </div>

          <div className="mode-toggle" style={{ marginBottom: '14px' }}>
            <button
              type="button"
              className={`mode-btn ${initialSchema === 1 ? 'active durable' : ''}`}
              onClick={() => setInitialSchema(1)}
            >
              📄 Start on Schema v1 (Compact Summary)
            </button>
            <button
              type="button"
              className={`mode-btn ${initialSchema === 2 ? 'active durable' : ''}`}
              onClick={() => setInitialSchema(2)}
            >
              📊 Start on Schema v2 (Executive Scorecard)
            </button>
          </div>

          <div style={{ marginBottom: '14px', display: 'flex', justifyContent: 'center' }}>
            <A2AChaosToggle outage={a2aOutage} toggling={togglingChaos} onToggle={toggleA2AChaos} />
          </div>

          <div className="form-field">
            <label htmlFor="council-architect">Release Architect</label>
            <input
              id="council-architect"
              type="text"
              value={architect}
              onChange={(e) => setArchitect(e.target.value)}
            />
          </div>
          <div className="form-field">
            <label htmlFor="council-release">Release Candidate</label>
            <input
              id="council-release"
              type="text"
              value={releaseName}
              onChange={(e) => setReleaseName(e.target.value)}
            />
          </div>
          <div className="form-field">
            <label htmlFor="council-env">Target Environment</label>
            <input
              id="council-env"
              type="text"
              value={environment}
              onChange={(e) => setEnvironment(e.target.value)}
            />
          </div>

          <button
            className="place-order-btn"
            onClick={startCouncil}
            disabled={loading || !releaseName.trim()}
          >
            {loading ? (
              <>
                <Loader size={18} className="spin" style={{ marginRight: '6px' }} /> Spawning 3 Concurrent A2A Audits...
              </>
            ) : (
              <>
                <Rocket size={18} style={{ marginRight: '6px' }} /> Fan-Out Parallel A2A Council
              </>
            )}
          </button>
        </div>
      </div>
    );
  }

  if (!workerAlive && workflowId) {
    return <WorkerDeadOverlay onRestart={restartWorker} restarting={restarting} downSince={downSince} />;
  }

  return (
    <div className={`main-layout ${devMode ? 'with-devtools' : ''}`}>
      <div className="main-content">
        <StatusBadge status={status} workflowId={workflowId} messageCount={messages.length} />

        <div className="controls">
          <button
            className="ctrl-btn primary"
            onClick={() => triggerUpgrade(currentSchema >= 2 ? 1 : 2)}
            disabled={status !== 'connected'}
            title="Hot-swap A2UI Verdict Surface Schema without restarting workflow (U)"
          >
            <ArrowUpCircle size={14} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />
            {currentSchema >= 2 ? '⇄ Switch to v1 Compact Schema (U)' : '🔼 Hot-Upgrade to v2 Executive Scorecard (U)'}
          </button>
          <button
            className="ctrl-btn approve"
            onClick={() => triggerDecision('approved')}
            disabled={status !== 'connected'}
            title="Approve Release Rollout (A)"
          >
            <CheckCircle size={14} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />
            Approve Rollout
          </button>
          <button
            className="ctrl-btn cancel"
            onClick={() => triggerDecision('rejected')}
            disabled={status !== 'connected'}
            title="Request Changes (X)"
          >
            <XCircle size={14} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />
            Request Changes
          </button>
          <A2AChaosToggle outage={a2aOutage} toggling={togglingChaos} onToggle={toggleA2AChaos} />
          <WorkerKillButton onKill={killWorker} killing={killing} workerAlive={workerAlive} />
          <QRShare workflowId={workflowId} />
          <button className="ctrl-btn" onClick={resetDemo} title="New council (N)">
            <RefreshCw size={14} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />
            New Council
          </button>
          <button
            className={`ctrl-btn devtools ${devMode ? 'active' : ''}`}
            onClick={() => setDevMode(!devMode)}
            title="Dev Mode (D)"
          >
            {'</>'} Dev
          </button>
        </div>

        <ConnectionBanner status={status} messageCount={messages.length} />

        <div className="agent-pipeline">
          {[
            { id: 'reliability', icon: <Activity />, label: 'SRE Telemetry (:9003)' },
            { id: 'secops', icon: <ShieldCheck />, label: 'SecOps Policy (:9004)' },
            { id: 'finops', icon: <Coins />, label: 'FinOps Quota (:9001)' },
            { id: 'verdict', icon: <Layers />, label: `Verdict Schema v${currentSchema}.0` },
          ].map((node) => {
            const isDone =
              node.id === 'verdict' ? completedSpecialists.size === 3 : completedSpecialists.has(node.id);
            const isActive = !isDone && status === 'connected';
            return (
              <div key={node.id} className={`agent-pipeline-item ${isDone ? 'completed' : ''}`}>
                <div className={`agent-node ${isActive ? 'active' : ''} ${isDone ? 'completed' : ''}`}>
                  {node.icon}
                  {isDone && (
                    <span className="completed-badge">
                      <CheckCircle />
                    </span>
                  )}
                </div>
                <span className="agent-node-label">{node.label}</span>
              </div>
            );
          })}
        </div>

        <ProtocolXRayStrip messages={messages} />

        {error && (
          <div className="error-toast" role="alert">
            {error}
            <button onClick={() => setError(null)} aria-label="Dismiss error">
              ×
            </button>
          </div>
        )}

        <div className="surfaces">
          {surfaces.map((surface) => (
            <div key={surface.id} className="surface-wrapper">
              <A2uiSurface surface={surface} />
            </div>
          ))}
          {surfaces.length === 0 && status === 'connected' && (
            <div className="loading">Spawning parallel workflow.Go A2A specialist evaluations...</div>
          )}
        </div>
      </div>

      {devMode && <DevToolsPanel messages={messages} onClose={() => setDevMode(false)} />}
    </div>
  );
}
