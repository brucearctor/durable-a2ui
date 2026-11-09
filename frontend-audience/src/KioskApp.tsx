/**
 * Minimal A2UI viewer for audience/kiosk mode.
 * Designed for phones, tablets, and second screens during demos.
 *
 * URL params:
 *   ?wf=workflow-id          — connect to an order tracking workflow
 *   ?wf=workflow-id&role=requester  — approval workflow, requester view
 *   ?wf=workflow-id&role=approver   — approval workflow, approver view
 */
import { useState, useEffect, useRef, useCallback } from 'react';
import { MessageProcessor, SurfaceModel } from '@a2ui/web_core/v0_9';
import { basicCatalog, A2uiSurface } from '@a2ui/react/v0_9';
import type { ReactComponentImplementation } from '@a2ui/react/v0_9';
import type { A2uiMessage, A2uiClientAction } from '@a2ui/web_core/v0_9';
import './KioskApp.css';

type ConnectionStatus = 'connecting' | 'connected' | 'reconnecting' | 'done';

function safeProcess(processor: MessageProcessor, msg: A2uiMessage): void {
  try {
    processor.processMessages([msg]);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    if (!message.includes('already exists')) {
      console.error('[A2UI] Process error:', err);
    }
  }
}

export default function KioskApp() {
  const params = new URLSearchParams(window.location.search);
  const workflowId = params.get('wf');
  const role = params.get('role');  // 'requester' | 'approver' | null (order tracking)

  const [surfaces, setSurfaces] = useState<SurfaceModel<ReactComponentImplementation>[]>([]);
  const [status, setStatus] = useState<ConnectionStatus>('connecting');
  const processorRef = useRef<MessageProcessor | null>(null);

  // Determine stream URL based on role
  const streamUrl = role
    ? `/api/approvals/${workflowId}/stream/${role}`
    : `/api/sessions/${workflowId}/stream`;

  const signalUrl = role
    ? `/api/approvals/${workflowId}/signal`
    : `/api/sessions/${workflowId}/signal`;

  // Send signal (for approver actions)
  const sendAction = useCallback(async (action: A2uiClientAction) => {
    if (!workflowId) return;
    fetch(`${signalUrl}/user_action`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(action),
    }).catch(console.error);
  }, [workflowId, signalUrl]);

  useEffect(() => {
    if (!workflowId) return;

    const processor = new MessageProcessor([basicCatalog], sendAction);
    processorRef.current = processor;

    const sub1 = processor.onSurfaceCreated((surface) => {
      setSurfaces((prev) => [...prev, surface]);
    });
    const sub2 = processor.onSurfaceDeleted((id) => {
      setSurfaces((prev) => prev.filter((s) => s.id !== id));
    });

    const es = new EventSource(streamUrl);

    es.onopen = () => setStatus('connected');

    es.onmessage = (event) => {
      const msg = JSON.parse(event.data) as A2uiMessage;
      safeProcess(processor, msg);
    };

    es.addEventListener('done', () => {
      setStatus('done');
      es.close();
    });

    es.onerror = () => setStatus('reconnecting');

    return () => {
      es.close();
      sub1.unsubscribe();
      sub2.unsubscribe();
    };
  }, [workflowId, streamUrl, sendAction]);

  // Role badge
  const roleLabel = role === 'approver' ? '👔 Approver' :
                    role === 'requester' ? '👤 Requester' :
                    '🍕 Live Order';

  if (!workflowId) {
    return (
      <div className="kiosk">
        <div className="kiosk-empty">
          <h1>Durable Generative UI</h1>
          <p>Scan a QR code or add <code>?wf=workflow-id</code> to the URL</p>
        </div>
      </div>
    );
  }

  return (
    <div className="kiosk">
      <header className="kiosk-header">
        <div className="kiosk-title">
          <span className="kiosk-role">{roleLabel}</span>
          <span className={`kiosk-dot ${status}`} />
        </div>
        <span className="kiosk-wf">{workflowId}</span>
      </header>

      <main className="kiosk-surface">
        {surfaces.map((surface) => (
          <A2uiSurface key={surface.id} surface={surface} />
        ))}
        {surfaces.length === 0 && status === 'connecting' && (
          <div className="kiosk-loading">Connecting to workflow...</div>
        )}
        {status === 'done' && (
          <div className="kiosk-done">✅ Workflow complete</div>
        )}
      </main>

      <footer className="kiosk-footer">
        <span>A2UI × Temporal</span>
      </footer>
    </div>
  );
}
