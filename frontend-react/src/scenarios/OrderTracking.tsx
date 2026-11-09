/**
 * Order Tracking scenario — the original demo, now with polish.
 * Uses shared components for chrome; owns its own form, hero, and state.
 * Includes: progress bar, narrative captions, auto-demo, kill-worker, surface flash.
 */
import { useState, useCallback, useEffect, useMemo, useRef } from 'react';
import { A2uiSurface } from '@a2ui/react';
import { useWorkflowStream } from '../hooks/useWorkflowStream';
import { StatusBadge, QRShare, EventLog, FragileOverlay, ControlsBar, ConnectionBanner } from '../components/shared';
import { StageProgressBar } from '../components/StageProgressBar';
import { useAutoDemo, AutoDemoOverlay, AutoDemoButton } from '../components/AutoDemo';
import { useWorkerStatus, WorkerKillButton, WorkerDeadOverlay } from '../components/WorkerStatus';

const DEFAULT_ITEMS = [
  { name: 'Margherita Pizza', qty: 2, price: 12.0 },
  { name: 'Caesar Salad', qty: 1, price: 8.5 },
  { name: 'Sparkling Water', qty: 2, price: 3.0 },
];

/* ── Narrative Caption ───────────────────────────────────────────── */

function Caption({ text }: { text: string }) {
  return <p className="narrative-caption">{text}</p>;
}

/* ── Hero ─────────────────────────────────────────────────────────── */

function HeroSection() {
  return (
    <div className="hero">
      <div className="hero-contrast">
        <div className="contrast-card fragile">
          <div className="contrast-icon">💥</div>
          <div className="contrast-label">Traditional UI</div>
          <div className="contrast-text">
            Refresh the page? <strong>Gone.</strong><br />
            Server restart? <strong>Gone.</strong><br />
            Two tabs? <strong>Out of sync.</strong>
          </div>
        </div>
        <div className="contrast-divider">→</div>
        <div className="contrast-card durable">
          <div className="contrast-icon">✅</div>
          <div className="contrast-label">This Demo</div>
          <div className="contrast-text">
            Refresh, disconnect, crash —<br />
            <strong>the UI survives.</strong><br />
            Because it's projected from durable workflow state.
          </div>
        </div>
      </div>
    </div>
  );
}

/* ── Component ───────────────────────────────────────────────────── */

export default function OrderTracking() {
  const urlParams = new URLSearchParams(window.location.search);
  const sharedWfId = urlParams.get('wf');

  const [workflowId, setWorkflowId] = useState<string | null>(sharedWfId);
  const [loading, setLoading] = useState(false);
  const [devMode, setDevMode] = useState(false);
  const [expandedMsg, setExpandedMsg] = useState<string | null>(null);
  const [fragileMode, setFragileMode] = useState(false);
  const [fragileTriggered, setFragileTriggered] = useState(false);
  const [customerName, setCustomerName] = useState('Bruce Arctor');
  const [clientCount, setClientCount] = useState(0);
  const [surfaceFlash, setSurfaceFlash] = useState(false);
  const prevMsgCount = useRef(0);

  // Handle A2UI button events from the rendered surface
  const handleAction = useCallback((eventName: string) => {
    if (eventName === 'reorder') {
      // Start a brand new workflow — full lifecycle demo
      fetch('/api/sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ customer: customerName, items: DEFAULT_ITEMS }),
      })
        .then((res) => res.json())
        .then((result) => setWorkflowId(result.workflow_id))
        .catch((err) => console.error('Reorder failed:', err));
      return true;
    }
    if (eventName === 'rate_order') {
      alert('⭐ Thanks for rating!');
      return true;
    }
    return false;
  }, [customerName]);

  const { surfaces, status, sendSignal, messages, simulateDisconnect } =
    useWorkflowStream(workflowId, { onAction: handleAction });

  // Refs to avoid stale closures in auto-demo
  const workflowIdRef = useRef(workflowId);
  workflowIdRef.current = workflowId;
  const sendSignalRef = useRef(sendSignal);
  sendSignalRef.current = sendSignal;
  const simulateDisconnectRef = useRef(simulateDisconnect);
  simulateDisconnectRef.current = simulateDisconnect;

  const { workerAlive, killing, restarting, killWorker, restartWorker, downSince } =
    useWorkerStatus();

  // Surface flash on new messages
  useEffect(() => {
    if (messages.length > prevMsgCount.current && prevMsgCount.current > 0) {
      setSurfaceFlash(true);
      const timer = setTimeout(() => setSurfaceFlash(false), 600);
      return () => clearTimeout(timer);
    }
    prevMsgCount.current = messages.length;
  }, [messages.length]);

  // Poll client count
  useEffect(() => {
    if (!workflowId) return;
    const interval = setInterval(async () => {
      try {
        const res = await fetch(`/api/sessions/${workflowId}/clients`);
        const data = await res.json();
        setClientCount(data.count);
      } catch { /* ignore */ }
    }, 3000);
    return () => clearInterval(interval);
  }, [workflowId]);

  // Update URL
  useEffect(() => {
    if (workflowId) {
      const url = new URL(window.location.href);
      url.searchParams.set('wf', workflowId);
      window.history.replaceState({}, '', url.toString());
    }
  }, [workflowId]);

  const placeOrder = useCallback(async (name?: string) => {
    setLoading(true);
    try {
      const res = await fetch('/api/sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ customer: name || customerName, items: DEFAULT_ITEMS }),
      });
      const result = await res.json();
      setWorkflowId(result.workflow_id);
      return result.workflow_id;
    } catch (err) {
      console.error('Failed to place order:', err);
    } finally {
      setLoading(false);
    }
  }, [customerName]);

  const resetDemo = useCallback(() => {
    setWorkflowId(null);
    setFragileMode(false);
    setFragileTriggered(false);
    const url = new URL(window.location.href);
    url.searchParams.delete('wf');
    window.history.replaceState({}, '', url.toString());
  }, []);

  const handleDisconnect = useCallback(() => {
    if (fragileMode) {
      setFragileTriggered(true);
    } else {
      simulateDisconnect();
    }
  }, [fragileMode, simulateDisconnect]);

  // Direct signal fetch — bypasses hook closure, always uses current workflowId
  const sendSignalDirect = useCallback(async (name: string) => {
    const wfId = workflowIdRef.current;
    if (!wfId) return;
    await fetch(`/api/sessions/${wfId}/signal/${name}`, { method: 'POST' });
  }, []);

  // Auto-demo steps — use refs and direct fetch to avoid stale closures
  const autoSteps = useMemo(() => [
    {
      label: 'Place Order',
      caption: '🚀 Starting a Temporal Workflow — UI will be projected from durable state',
      action: () => placeOrder('Demo User'),
      delayAfter: 3000,
    },
    {
      label: 'Fast Forward',
      caption: '⏩ Sending fast_forward signal — each stage publishes incremental A2UI updates',
      action: () => sendSignalDirect('fast_forward'),
      delayAfter: 2500,
    },
    {
      label: 'Fast Forward',
      caption: '⏩ Confirmed → Preparing — the modify button becomes disabled via updateComponents',
      action: () => sendSignalDirect('fast_forward'),
      delayAfter: 2500,
    },
    {
      label: 'Disconnect',
      caption: '🔌 SSE connection closed — simulating a network failure or page refresh',
      action: () => simulateDisconnectRef.current(),
      delayAfter: 3000,
    },
    {
      label: 'Reconnecting',
      caption: '🔄 Reconnecting from offset 0 — the full stream replays and UI rebuilds identically',
      action: () => {}, // reconnect happens automatically
      delayAfter: 3000,
    },
    {
      label: 'Fast Forward',
      caption: '⏩ Shipped — driver assigned, tracking number appears, cancel button disabled',
      action: () => sendSignalDirect('fast_forward'),
      delayAfter: 2500,
    },
    {
      label: 'Delivered',
      caption: '✅ Delivered — actions replaced with Rate & Reorder. Workflow complete.',
      action: () => sendSignalDirect('fast_forward'),
      delayAfter: 3000,
    },
  ], [placeOrder, sendSignalDirect]);

  const autoDemo = useAutoDemo(autoSteps);

  // Keyboard shortcuts for live demo
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // Don't capture when typing in inputs
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;

      switch (e.key.toLowerCase()) {
        case 'f': sendSignalDirect('fast_forward'); break;
        case 'd': simulateDisconnectRef.current(); break;
        case 'k': if (workerAlive) killWorker(); break;
        case 'r': if (!workerAlive) restartWorker(); break;
        case 'n': resetDemo(); break;
        case ' ':
          e.preventDefault();
          if (!autoDemo.running) autoDemo.start();
          else autoDemo.stop();
          break;
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [sendSignalDirect, workerAlive, killWorker, restartWorker, resetDemo, autoDemo]);

  if (!workflowId && !autoDemo.running) {
    return (
      <div className="start-section">
        <HeroSection />
        <div className="order-form">
          <h2>🍕 Try It Yourself</h2>
          <div className="form-actions-row">
            <AutoDemoButton onClick={autoDemo.start} />
            <span className="form-or">or fill in below</span>
          </div>
          <div className="form-field">
            <label htmlFor="customer-name">Your Name</label>
            <input
              id="customer-name"
              type="text"
              value={customerName}
              onChange={(e) => setCustomerName(e.target.value)}
              placeholder="Enter your name"
            />
          </div>
          <div className="order-items-preview">
            {DEFAULT_ITEMS.map((item, i) => (
              <div key={i} className="item-row">
                <span>{item.name} × {item.qty}</span>
                <span className="item-price">${(item.price * item.qty).toFixed(2)}</span>
              </div>
            ))}
            <div className="item-total">
              ${DEFAULT_ITEMS.reduce((s, i) => s + i.price * i.qty, 0).toFixed(2)}
            </div>
          </div>
          <button
            className="place-order-btn"
            onClick={() => placeOrder()}
            disabled={loading || !customerName.trim()}
          >
            {loading ? '⏳ Starting workflow...' : '🚀 Place Order & Watch'}
          </button>
        </div>
      </div>
    );
  }

  if (fragileTriggered) {
    return (
      <FragileOverlay onSwitchBack={() => {
        setFragileTriggered(false);
        setFragileMode(false);
      }} />
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
        {autoDemo.running && (
          <AutoDemoOverlay
            caption={autoDemo.caption}
            step={autoDemo.currentStep}
            total={autoSteps.length}
            onStop={autoDemo.stop}
          />
        )}

        <StatusBadge status={status} workflowId={workflowId!} clientCount={clientCount} messageCount={messages.length} />

        <ControlsBar
          status={status}
          fragileMode={fragileMode}
          devMode={devMode}
          onFragileToggle={setFragileMode}
          onDevToggle={() => setDevMode(!devMode)}
          onFastForward={() => sendSignal('fast_forward')}
          onDisconnect={handleDisconnect}
          onReset={resetDemo}
        />

        {/* Kill-worker control */}
        {status !== 'idle' && (
          <div className="worker-controls">
            <WorkerKillButton onKill={killWorker} killing={killing} workerAlive={workerAlive} />
            <Caption text="Kill the Temporal Worker process — the workflow survives on the server" />
          </div>
        )}

        <div className="share-bar">
          <QRShare workflowId={workflowId!} />
          <span className="share-hint">💡 Open this URL in another tab — same live workflow</span>
        </div>

        {/* Progress bar */}
        {workflowId && (
          <div className="progress-section">
            <StageProgressBar messages={messages} />
            <Caption text="Each stage is an incremental A2UI update — not a full re-render" />
          </div>
        )}

        <ConnectionBanner status={status} messageCount={messages.length} />

        <section className={`surfaces ${surfaceFlash ? 'surface-flash' : ''}`}>
          {surfaces.map((surface) => (
            <div key={surface.id} className="surface-wrapper">
              <A2uiSurface surface={surface} />
            </div>
          ))}
        </section>
      </div>

      {devMode && (
        <div className="devtools-panel">
          <EventLog
            messages={messages}
            expanded={expandedMsg}
            onToggle={(id) => setExpandedMsg(expandedMsg === id ? null : id)}
          />
          <Caption text="Every message is persisted in Temporal's event history." />
        </div>
      )}
    </div>
  );
}
