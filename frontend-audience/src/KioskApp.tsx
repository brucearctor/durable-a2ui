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
import { MessageProcessor, SurfaceModel, basicCatalog } from '@a2ui/web_core/v1_0';
import { basicCatalog as basicCatalogV09 } from '@a2ui/web_core/v0_9';
import { setMarkdownRenderer } from '@a2ui/web_core/v0_9/basic_catalog';
import { renderMarkdown } from '@a2ui/markdown-it';
import { A2uiSurface } from '@a2ui/react';
import type { ReactComponentImplementation } from '@a2ui/react';
import type { A2uiMessage, A2uiClientAction } from '@a2ui/web_core/v0_9';
import './KioskApp.css';

setMarkdownRenderer(renderMarkdown);

type ConnectionStatus = 'connecting' | 'connected' | 'reconnecting' | 'done';

function safeProcess(processor: MessageProcessor<any>, msg: A2uiMessage): void {
  try {
    if ('createSurface' in msg) {
      const cs = (msg as any).createSurface;
      if (cs?.surfaceId && processor.getSurface(cs.surfaceId)) {
        processor.processMessages([
          {
            version: (msg as any).version || '1.0',
            deleteSurface: { surfaceId: cs.surfaceId },
          } as any,
        ]);
      }
    }
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
  const processorRef = useRef<MessageProcessor<any> | null>(null);

  // Determine stream URL based on role and workflow ID prefix
  let streamUrl = `/api/orders/${workflowId}/stream`;
  let signalUrl = `/api/orders/${workflowId}/signal`;

  if (role) {
    streamUrl = `/api/approvals/${workflowId}/stream/${role}`;
    signalUrl = `/api/approvals/${workflowId}/signal`;
  } else if (workflowId?.startsWith('approval-')) {
    streamUrl = `/api/approvals/${workflowId}/stream/approver`;
    signalUrl = `/api/approvals/${workflowId}/signal`;
  } else if (workflowId?.startsWith('concierge-')) {
    streamUrl = `/api/concierge/${workflowId}/stream`;
    signalUrl = `/api/concierge/${workflowId}/message`;
  } else if (workflowId?.startsWith('incident-')) {
    streamUrl = `/api/incidents/${workflowId}/stream`;
    signalUrl = `/api/incidents/${workflowId}/approve`;
  } else if (workflowId?.startsWith('council-')) {
    streamUrl = `/api/council/${workflowId}/stream`;
    signalUrl = `/api/council/${workflowId}/decide`;
  } else if (workflowId?.startsWith('order-')) {
    streamUrl = `/api/orders/${workflowId}/stream`;
    signalUrl = `/api/orders/${workflowId}/signal`;
  }

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

    const processor = new MessageProcessor<ReactComponentImplementation>([basicCatalog as any, basicCatalogV09 as any], sendAction);
    processorRef.current = processor;

    const sub1 = processor.onSurfaceCreated((surface: SurfaceModel<ReactComponentImplementation>) => {
      setSurfaces((prev) => [...prev.filter((s) => s.id !== surface.id), surface]);
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
                    workflowId?.startsWith('concierge-') ? '🍽️ AI Concierge' :
                    workflowId?.startsWith('incident-') ? '🚨 Incident Commander' :
                    workflowId?.startsWith('council-') ? '🏛️ A2A Council' :
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
