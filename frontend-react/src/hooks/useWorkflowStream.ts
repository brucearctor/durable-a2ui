import { useState, useEffect, useCallback, useRef } from 'react';
import { MessageProcessor, SurfaceModel, basicCatalog } from '@a2ui/web_core/v1_0';
import { basicCatalog as basicCatalogV09 } from '@a2ui/web_core/v0_9';
import { setMarkdownRenderer } from '@a2ui/web_core/v0_9/basic_catalog';
import { renderMarkdown } from '@a2ui/markdown-it';
import type { ReactComponentImplementation } from '@a2ui/react';
import type { A2uiMessage, A2uiClientAction } from '@a2ui/web_core/v0_9';

// Register markdown-it renderer for A2UI v1.0 Text components
setMarkdownRenderer(renderMarkdown);

/**
 * useWorkflowStream — React hook for consuming A2UI WorkflowStream events.
 *
 * Data flow:
 *   Server (SSE) ──► EventSource ──► A2UI MessageProcessor ──► React state
 *
 * Key behaviors:
 *   - Auto-reconnects on disconnect (1s backoff)
 *   - Replays full stream on reconnect (Temporal delivers full history)
 *   - Ignores "already exists" errors during replay (idempotent surfaces)
 *   - Extracts data model updates for application state (messages, cart, etc.)
 *   - Provides connection status (connected/reconnecting/disconnected)
 */

export type StreamStatus = 'idle' | 'connected' | 'reconnecting' | 'disconnected' | 'done';

export interface MessageLogEntry {
  id: string;
  timestamp: number;
  type: string;
  summary: string;
  data: unknown;
}

function classifyMessage(msg: A2uiMessage): { type: string; summary: string } {
  if ('createSurface' in msg) {
    const cs = (msg as any).createSurface;
    return { type: 'createSurface', summary: `Surface "${cs.surfaceId}"` };
  }
  if ('updateComponents' in msg) {
    const uc = (msg as any).updateComponents;
    const count = uc.components?.length ?? 0;
    return { type: 'updateComponents', summary: `${count} component${count !== 1 ? 's' : ''}` };
  }
  if ('updateDataModel' in msg) {
    const ud = (msg as any).updateDataModel;
    const keys = ud.value ? Object.keys(ud.value) : [];
    return { type: 'updateDataModel', summary: keys.join(', ') || '/' };
  }
  if ('deleteSurface' in msg) {
    return { type: 'deleteSurface', summary: (msg as any).deleteSurface.surfaceId };
  }
  return { type: 'unknown', summary: JSON.stringify(msg).slice(0, 60) };
}

/**
 * Safely process an A2UI message, replacing existing surfaces when re-created
 * and ignoring benign replay errors.
 */
function safeProcess(
  processor: MessageProcessor<any>,
  msg: A2uiMessage,
  isReplay = false,
): void {
  try {
    if ('createSurface' in msg) {
      const cs = (msg as any).createSurface;
      if (cs?.surfaceId && processor.getSurface(cs.surfaceId)) {
        if (isReplay) {
          return;
        }
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
    if (message.includes('already exists')) {
      console.debug('[A2UI] Skipping duplicate surface on replay');
    } else {
      console.error('[A2UI] Failed to process message:', err, msg);
    }
  }
}

function createProcessor(
  workflowIdRef: React.RefObject<string | null>,
  optionsRef: React.RefObject<StreamOptions | undefined>,
): MessageProcessor<any> {
  return new MessageProcessor([basicCatalog as any, basicCatalogV09 as any], (action: A2uiClientAction) => {
    const id = workflowIdRef.current;
    if (!id) return;
    console.log('[A2UI] User action:', action);

    // Let component handle the action if it wants to
    const eventName = (action as any)?.event?.name || (action as any)?.name;
    const onAction = optionsRef.current?.onAction;
    if (onAction && eventName) {
      const handled = onAction(eventName, action);
      if (handled) return;
    }

    const signalBase = optionsRef.current?.signalUrl || `/api/sessions/${id}/signal`;
    fetch(`${signalBase}/user_action`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(action),
    }).catch((err) => console.error('Failed to send action:', err));
  });
}

export interface StreamOptions {
  /** Override the SSE stream URL (default: /api/sessions/{id}/stream) */
  streamUrl?: string;
  /** Override the signal URL prefix (default: /api/sessions/{id}/signal) */
  signalUrl?: string;
  /** Intercept A2UI button actions. Return true to prevent default signal send. */
  onAction?: (eventName: string, action: A2uiClientAction) => boolean;
}

export function useWorkflowStream(workflowId: string | null, options?: StreamOptions) {
  const workflowIdRef = useRef(workflowId);
  workflowIdRef.current = workflowId;
  const optionsRef = useRef(options);
  optionsRef.current = options;

  // Fresh processor per workflow — avoids "surface already exists" when switching orders
  const processorRef = useRef<MessageProcessor<ReactComponentImplementation> | null>(null);

  const [surfaces, setSurfaces] = useState<SurfaceModel<ReactComponentImplementation>[]>([]);
  const [status, setStatus] = useState<StreamStatus>('idle');
  const [messages, setMessages] = useState<MessageLogEntry[]>([]);
  const esRef = useRef<EventSource | null>(null);

  const sendSignal = useCallback(
    async (signalName: string, payload?: unknown) => {
      if (!workflowId) return;
      const signalBase = options?.signalUrl || `/api/sessions/${workflowId}/signal`;
      await fetch(`${signalBase}/${signalName}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload !== undefined ? JSON.stringify(payload) : undefined,
      });
    },
    [workflowId, options?.signalUrl],
  );

  // Simulate disconnect for durability demo
  const simulateDisconnect = useCallback(() => {
    if (esRef.current) {
      console.log('[SSE] Simulating disconnect');
      esRef.current.close();
      esRef.current = null;
      setStatus('disconnected');

      // Reconnect after 2 seconds
      setTimeout(() => {
        const wfId = workflowIdRef.current;
        const processor = processorRef.current;
        if (!wfId || !processor) return;

        console.log('[SSE] Reconnecting...');
        setStatus('reconnecting');

        // Reconnect from offset 0 — the processor handles duplicates gracefully
        const streamUrl = optionsRef.current?.streamUrl || `/api/sessions/${wfId}/stream`;
        const es = new EventSource(streamUrl);

        es.onopen = () => {
          console.log('[SSE] Reconnected!');
          setStatus('connected');
        };

        es.onmessage = (event) => {
          const msg = JSON.parse(event.data) as A2uiMessage;
          const { type, summary } = classifyMessage(msg);
          console.log('[SSE] Replay:', event.lastEventId, type);
          setMessages((prev) => {
            // Deduplicate by event ID
            if (prev.some((m) => m.id === event.lastEventId)) return prev;
            return [...prev, { id: event.lastEventId, timestamp: Date.now(), type, summary, data: msg }];
          });
          safeProcess(processor, msg, true);
        };

        es.addEventListener('done', (event) => {
          console.log('[SSE] Workflow completed:', event.data);
          setStatus('done');
          es.close();
        });

        es.onerror = () => setStatus('reconnecting');
        esRef.current = es;
      }, 2000);
    }
  }, []);

  useEffect(() => {
    if (!workflowId) {
      setStatus('idle');
      setSurfaces([]);
      setMessages([]);
      return;
    }

    // Create fresh processor for this workflow
    const processor = createProcessor(workflowIdRef, optionsRef);
    processorRef.current = processor;
    setMessages([]);
    setSurfaces([]);

    const sub1 = processor.onSurfaceCreated((surface: SurfaceModel<ReactComponentImplementation>) => {
      console.log('[A2UI] Surface created:', surface.id);
      setSurfaces((prev) => [...prev.filter((s) => s.id !== surface.id), surface]);
    });
    const sub2 = processor.onSurfaceDeleted((id: string) => {
      console.log('[A2UI] Surface deleted:', id);
      setSurfaces((prev) => prev.filter((s) => s.id !== id));
    });

    const streamUrl = options?.streamUrl || `/api/sessions/${workflowId}/stream`;
    const es = new EventSource(streamUrl);
    esRef.current = es;

    es.onopen = () => {
      console.log('[SSE] Connected');
      setStatus('connected');
    };

    es.onmessage = (event) => {
      const msg = JSON.parse(event.data) as A2uiMessage;
      const { type, summary } = classifyMessage(msg);
      console.log('[SSE] Message:', event.lastEventId, type);
      setMessages((prev) => [
        ...prev,
        { id: event.lastEventId, timestamp: Date.now(), type, summary, data: msg },
      ]);
      safeProcess(processor, msg);
    };

    es.addEventListener('done', (event) => {
      console.log('[SSE] Workflow completed:', event.data);
      setStatus('done');
      es.close();
    });

    es.onerror = () => setStatus('reconnecting');

    return () => {
      console.log('[SSE] Closing connection');
      es.close();
      esRef.current = null;
      sub1.unsubscribe();
      sub2.unsubscribe();
    };
  }, [workflowId, options?.streamUrl]);

  return { surfaces, status, sendSignal, messages, simulateDisconnect };
}
