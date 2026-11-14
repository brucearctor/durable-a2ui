import { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import { A2uiSurface } from '@a2ui/react';
import { ClipboardList, User, Briefcase, CheckCircle, XCircle, RefreshCw, Plus, Trash2, Loader, Send } from 'lucide-react';
import { useWorkflowStream } from '../hooks/useWorkflowStream';
import type { StreamOptions } from '../hooks/useWorkflowStream';
import { StatusBadge, ConnectionBanner, QRShare } from '../components/shared';
import { useWorkerStatus, WorkerKillButton, WorkerDeadOverlay } from '../components/WorkerStatus';
import DevToolsPanel from '../components/DevToolsPanel';

interface ExpenseItem {
  description: string;
  amount: number;
  category: string;
}

const PRESETS = {
  cloud_infra: {
    requester: 'Sarah Chen (Platform Engineering Lead)',
    title: 'Production AI Inference — GKE L4 GPU Cluster & AlloyDB Quota Grant',
    justification:
      'Provisioning dedicated GKE L4 GPU inference nodes, AlloyDB read-replica capacity, and OpenTelemetry trace retention for multi-agent workloads.',
    items: [
      { description: 'GKE L4 GPU Inference Node Pool (Multi-Zone)', amount: 680, category: 'Cloud Compute' },
      { description: 'AlloyDB Read-Replica High-Availability Pool', amount: 720, category: 'Database' },
      { description: 'OpenTelemetry & Distributed Trace Retention Quota', amount: 485, category: 'Observability' },
      { description: 'On-Call Engineering Launch War-Room Catering', amount: 250, category: 'Operations' },
    ],
  },
  campus_hackathon: {
    requester: 'Alex Rivera (ACM Student Chapter Lead)',
    title: 'University AI Hackathon — GPU Compute Grant & Hardware Kit',
    justification:
      'University student hackathon building durable multi-agent systems with Go, ADK, A2A, and Temporal.',
    items: [
      { description: 'Cloud L4 GPU Credits for 40 Student Teams', amount: 950, category: 'Cloud Compute' },
      { description: 'Raspberry Pi 5 & Edge AI Robotics Kits (×6)', amount: 660, category: 'Hardware' },
      { description: 'Late-Night Boba & Pizza Lab Fuel', amount: 325, category: 'Catering' },
      { description: 'Winning Team Conference Travel Stipend', amount: 200, category: 'Prizes' },
    ],
  },
};

export default function ApprovalWorkflow() {
  const [workflowId, setWorkflowId] = useState<string | null>(null);
  const [devMode, setDevMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (error) {
      const t = setTimeout(() => setError(null), 5000);
      return () => clearTimeout(t);
    }
  }, [error]);

  const [presetKey, setPresetKey] = useState<'cloud_infra' | 'campus_hackathon'>('cloud_infra');
  const [requester, setRequester] = useState(PRESETS.cloud_infra.requester);
  const [title, setTitle] = useState(PRESETS.cloud_infra.title);
  const [justification, setJustification] = useState(PRESETS.cloud_infra.justification);
  const [items, setItems] = useState<ExpenseItem[]>(PRESETS.cloud_infra.items);

  const applyPreset = (key: 'cloud_infra' | 'campus_hackathon') => {
    setPresetKey(key);
    const p = PRESETS[key];
    setRequester(p.requester);
    setTitle(p.title);
    setJustification(p.justification);
    setItems(p.items);
  };

  const requesterOpts = useMemo<StreamOptions | undefined>(() =>
    workflowId ? {
      streamUrl: `/api/approvals/${workflowId}/stream/requester`,
      signalUrl: `/api/approvals/${workflowId}/signal`,
    } : undefined,
    [workflowId]
  );

  const approverOpts = useMemo<StreamOptions | undefined>(() =>
    workflowId ? {
      streamUrl: `/api/approvals/${workflowId}/stream/approver`,
      signalUrl: `/api/approvals/${workflowId}/signal`,
    } : undefined,
    [workflowId]
  );

  const requesterStream = useWorkflowStream(workflowId, requesterOpts);
  const approverStream = useWorkflowStream(workflowId, approverOpts);
  const { workerAlive, killing, restarting, killWorker, restartWorker, downSince } = useWorkerStatus();

  const allMessages = useMemo(() => {
    return [...requesterStream.messages, ...approverStream.messages]
      .sort((a, b) => a.timestamp - b.timestamp);
  }, [requesterStream.messages, approverStream.messages]);

  const totalAmount = items.reduce((s, item) => s + item.amount, 0);

  const workflowIdRef = useRef(workflowId);
  workflowIdRef.current = workflowId;

  const sendSignalDirect = useCallback(async (name: string) => {
    const wfId = workflowIdRef.current;
    if (!wfId) return;
    await fetch(`/api/approvals/${wfId}/signal/${name}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ note: `${name} via presenter shortcut` }),
    });
  }, []);

  const submitExpense = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/approvals', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ requester, title, justification, items, scenario: presetKey }),
      });
      const result = await res.json();
      setWorkflowId(result.workflow_id);
    } catch (err) {
      console.error('Failed to start approval workflow:', err);
      setError('Could not submit expense. Is the Go backend running?');
    } finally {
      setLoading(false);
    }
  }, [requester, title, justification, items, presetKey]);

  const resetDemo = useCallback(() => {
    setWorkflowId(null);
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;

      switch (e.key.toLowerCase()) {
        case 'a': sendSignalDirect('approve'); break;
        case 'x': sendSignalDirect('reject'); break;
        case 'n': resetDemo(); break;
        case 'd': setDevMode(prev => !prev); break;
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [sendSignalDirect, resetDemo]);

  const addItem = () => {
    setItems([...items, { description: '', amount: 0, category: 'Other' }]);
  };
  const removeItem = (idx: number) => {
    if (items.length <= 1) return;
    setItems(items.filter((_, i) => i !== idx));
  };
  const updateItem = (idx: number, field: keyof ExpenseItem, val: string | number) => {
    const updated = [...items];
    if (field === 'amount') {
      const num = Number(val);
      updated[idx] = { ...updated[idx], amount: Number.isNaN(num) ? 0 : Math.max(0, num) };
    } else {
      updated[idx] = { ...updated[idx], [field]: val as string };
    }
    setItems(updated);
  };

  if (!workflowId) {
    return (
      <div className="start-section">
        <div className="order-form approval-form">
          <h2><ClipboardList size={20} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '8px' }} />Submit Dual-Surface Approval Request</h2>

          <div className="mode-toggle" style={{ marginBottom: '14px' }}>
            <button
              type="button"
              className={`mode-btn ${presetKey === 'cloud_infra' ? 'active durable' : ''}`}
              onClick={() => applyPreset('cloud_infra')}
            >
              ☁️ Production AI & GPU Cluster Grant
            </button>
            <button
              type="button"
              className={`mode-btn ${presetKey === 'campus_hackathon' ? 'active durable' : ''}`}
              onClick={() => applyPreset('campus_hackathon')}
            >
              🎓 University Campus GPU & Hackathon Grant
            </button>
          </div>

          <div className="form-field">
            <label htmlFor="expense-requester">Your Name</label>
            <input id="expense-requester" type="text" value={requester} onChange={e => setRequester(e.target.value)} />
          </div>
          <div className="form-field">
            <label htmlFor="expense-title">Request Title</label>
            <input id="expense-title" type="text" value={title} onChange={e => setTitle(e.target.value)} />
          </div>
          <div className="form-field">
            <label htmlFor="expense-justification">Justification</label>
            <textarea id="expense-justification" value={justification} onChange={e => setJustification(e.target.value)} rows={3} />
          </div>
          <div className="items-section">
            <div className="items-header">
              <label>Line Items</label>
              <button type="button" className="add-item-btn" onClick={addItem}><Plus size={14} style={{ display: 'inline', verticalAlign: 'text-bottom' }} /> Add Item</button>
            </div>
            {items.map((item, idx) => (
              <div key={idx} className="item-row">
                <input
                  type="text"
                  placeholder="Description"
                  aria-label={`Item ${idx + 1} description`}
                  value={item.description}
                  onChange={e => updateItem(idx, 'description', e.target.value)}
                  className="item-name"
                />
                <input
                  type="text"
                  placeholder="Category"
                  aria-label={`Item ${idx + 1} category`}
                  value={item.category}
                  onChange={e => updateItem(idx, 'category', e.target.value)}
                  className="item-category"
                />
                <span className="price-prefix">$</span>
                <input
                  type="number"
                  placeholder="Amount"
                  aria-label={`Item ${idx + 1} amount`}
                  value={item.amount}
                  min={0}
                  step={10}
                  onChange={e => updateItem(idx, 'amount', e.target.value)}
                  className="item-price"
                />
                {items.length > 1 && (
                  <button type="button" className="remove-item-btn" onClick={() => removeItem(idx)} aria-label={`Remove item ${idx + 1}`}>
                    <Trash2 size={14} aria-hidden="true" />
                  </button>
                )}
              </div>
            ))}
          </div>
          <div className="order-total">
            Total: <strong>${totalAmount.toFixed(2)}</strong>
          </div>
          <button
            className="place-order-btn"
            onClick={submitExpense}
            disabled={loading || !requester.trim() || !title.trim() || items.some(i => !i.description.trim())}
          >
            {loading ? (
              <><Loader size={18} className="spin" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '6px' }} /> Submitting...</>
            ) : (
              <><Send size={18} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '6px' }} /> Submit for Dual-Surface Approval</>
            )}
          </button>
          {error && (
            <div className="error-toast" role="alert">
              {error}
              <button onClick={() => setError(null)} aria-label="Dismiss error">×</button>
            </div>
          )}
        </div>
      </div>
    );
  }

  if (!workerAlive && workflowId) {
    return (
      <WorkerDeadOverlay onRestart={restartWorker} restarting={restarting} downSince={downSince} />
    );
  }

  return (
    <div className={`main-layout ${devMode ? 'with-devtools' : ''}`}>
      <div className="main-content">
        <StatusBadge
          status={requesterStream.status}
          workflowId={workflowId}
          messageCount={allMessages.length}
        />

        <div className="controls">
          <button
            className="ctrl-btn approve"
            onClick={() => approverStream.sendSignal('approve')}
            disabled={approverStream.status !== 'connected'}
            title="Approve (A)"
          >
            <CheckCircle size={14} style={{ display: 'inline', verticalAlign: 'text-bottom' }} /> Approve
          </button>
          <button
            className="ctrl-btn cancel"
            onClick={() => approverStream.sendSignal('reject')}
            disabled={approverStream.status !== 'connected'}
            title="Reject (X)"
          >
            <XCircle size={14} style={{ display: 'inline', verticalAlign: 'text-bottom' }} /> Reject
          </button>
          <WorkerKillButton onKill={killWorker} killing={killing} workerAlive={workerAlive} />
          <button className="ctrl-btn" onClick={resetDemo} title="New request (N)">
            <RefreshCw size={14} style={{ display: 'inline', verticalAlign: 'text-bottom' }} /> New Request
          </button>
          <button className={`ctrl-btn devtools ${devMode ? 'active' : ''}`} onClick={() => setDevMode(!devMode)} title="Dev Mode (D)">
            {'</>'} Dev
          </button>
        </div>

        <ConnectionBanner status={requesterStream.status} messageCount={allMessages.length} />

        <div className="dual-surface">
          <div className="surface-pane">
            <div className="surface-pane-header">
              <span className="surface-pane-icon"><User size={18} /></span>
              <div>
                <h3>Requester View ({presetKey === 'campus_hackathon' ? 'Student Lead' : 'Tech Lead'})</h3>
                <span className="surface-pane-sub">{requester} — Topic: <code>requester-ui</code></span>
              </div>
              <QRShare workflowId={workflowId} role="requester" />
            </div>
            <div className="surface-pane-body">
              {requesterStream.surfaces.map(surface => (
                <div key={surface.id} className="surface-wrapper">
                  <A2uiSurface surface={surface} />
                </div>
              ))}
              {requesterStream.surfaces.length === 0 && requesterStream.status === 'connected' && (
                <div className="loading">Waiting for requester surface...</div>
              )}
            </div>
          </div>

          <div className="surface-pane">
            <div className="surface-pane-header">
              <span className="surface-pane-icon"><Briefcase size={18} /></span>
              <div>
                <h3>Approver View ({presetKey === 'campus_hackathon' ? 'Faculty Advisor' : 'VP Engineering'})</h3>
                <span className="surface-pane-sub">Topic: <code>approver-ui</code> (Interactive Buttons)</span>
              </div>
              <QRShare workflowId={workflowId} role="approver" />
            </div>
            <div className="surface-pane-body">
              {approverStream.surfaces.map(surface => (
                <div key={surface.id} className="surface-wrapper">
                  <A2uiSurface surface={surface} />
                </div>
              ))}
              {approverStream.surfaces.length === 0 && approverStream.status === 'connected' && (
                <div className="loading">Waiting for approver surface...</div>
              )}
            </div>
          </div>
        </div>
      </div>

      {devMode && <DevToolsPanel messages={allMessages} onClose={() => setDevMode(false)} />}
    </div>
  );
}
