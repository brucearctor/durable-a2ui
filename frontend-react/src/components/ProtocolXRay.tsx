import { useState } from 'react';
import { Radio, ChevronDown, ChevronRight, RefreshCw, Layers, ShieldAlert, Code2 } from 'lucide-react';

export interface A2AXRayMeta {
  agent_name: string;
  endpoint: string;
  attempt: number;
  agent_card?: Record<string, unknown>;
  request?: Record<string, unknown>;
  response?: Record<string, unknown>;
}

export interface TraceWithXRay {
  type: string;
  function?: string;
  specialist?: string;
  status?: string;
  timestamp?: number;
  xray?: A2AXRayMeta;
  a2ui_payloads?: Record<string, unknown>[];
  [key: string]: unknown;
}

export function ProtocolXRayCard({
  trace,
  defaultOpen = false,
}: {
  trace: TraceWithXRay;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const [activeTab, setActiveTab] = useState<'rpc' | 'card' | 'a2ui'>('rpc');

  const xray = trace.xray;
  const payloads = trace.a2ui_payloads || [];
  if (!xray && payloads.length === 0) return null;

  const label = trace.function || trace.specialist || xray?.agent_name || trace.type;
  const attempt = xray?.attempt || 1;

  return (
    <div
      className="xray-card"
      style={{
        background: 'rgba(10, 12, 24, 0.78)',
        border: attempt > 1 ? '1px solid rgba(251, 191, 36, 0.45)' : '1px solid rgba(124, 92, 252, 0.28)',
        borderRadius: '10px',
        marginBottom: '8px',
        overflow: 'hidden',
        textAlign: 'left',
      }}
    >
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        style={{
          width: '100%',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          padding: '8px 12px',
          background: 'transparent',
          border: 'none',
          color: 'var(--text)',
          cursor: 'pointer',
          fontSize: '0.76rem',
          fontFamily: 'var(--mono)',
          textAlign: 'left',
          flexWrap: 'wrap',
        }}
      >
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <span style={{ fontWeight: 700, color: '#c4b5fd' }}>{label}</span>

        {xray && (
          <span
            style={{
              padding: '2px 7px',
              borderRadius: '999px',
              background: 'rgba(59, 130, 246, 0.16)',
              border: '1px solid rgba(59, 130, 246, 0.35)',
              color: '#93c5fd',
              fontSize: '0.66rem',
            }}
          >
            🔗 A2A {xray.endpoint.replace('http://localhost', '')}
          </span>
        )}

        {xray && (
          <span
            style={{
              padding: '2px 7px',
              borderRadius: '999px',
              background: attempt > 1 ? 'rgba(251, 191, 36, 0.2)' : 'rgba(16, 185, 129, 0.14)',
              border: attempt > 1 ? '1px solid rgba(251, 191, 36, 0.5)' : '1px solid rgba(16, 185, 129, 0.3)',
              color: attempt > 1 ? '#fde68a' : '#6ee7b7',
              fontSize: '0.66rem',
              fontWeight: attempt > 1 ? 700 : 500,
            }}
          >
            {attempt > 1 ? `⚡ Recovered on Attempt #${attempt}` : `Attempt #${attempt}`}
          </span>
        )}

        {payloads.length > 0 && (
          <span
            style={{
              padding: '2px 7px',
              borderRadius: '999px',
              background: 'rgba(167, 139, 250, 0.15)',
              border: '1px solid rgba(167, 139, 250, 0.35)',
              color: '#ddd6fe',
              fontSize: '0.66rem',
            }}
          >
            ⚡ A2UI v1.0 ({payloads.length} {payloads.length === 1 ? 'msg' : 'msgs'})
          </span>
        )}
      </button>

      {open && (
        <div style={{ borderTop: '1px solid rgba(255, 255, 255, 0.08)', padding: '10px 12px' }}>
          <div style={{ display: 'flex', gap: '6px', marginBottom: '8px', flexWrap: 'wrap' }}>
            {xray && (
              <>
                <button
                  type="button"
                  onClick={() => setActiveTab('rpc')}
                  style={{
                    padding: '4px 10px',
                    fontSize: '0.68rem',
                    borderRadius: '6px',
                    border: '1px solid rgba(255,255,255,0.12)',
                    background: activeTab === 'rpc' ? 'rgba(124, 92, 252, 0.28)' : 'rgba(255,255,255,0.04)',
                    color: activeTab === 'rpc' ? '#fff' : 'var(--text-2)',
                    cursor: 'pointer',
                  }}
                >
                  <Code2 size={11} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />
                  A2A JSON-RPC 2.0 (message/send)
                </button>
                <button
                  type="button"
                  onClick={() => setActiveTab('card')}
                  style={{
                    padding: '4px 10px',
                    fontSize: '0.68rem',
                    borderRadius: '6px',
                    border: '1px solid rgba(255,255,255,0.12)',
                    background: activeTab === 'card' ? 'rgba(59, 130, 246, 0.28)' : 'rgba(255,255,255,0.04)',
                    color: activeTab === 'card' ? '#fff' : 'var(--text-2)',
                    cursor: 'pointer',
                  }}
                >
                  <Radio size={11} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />
                  /.well-known/agent-card.json
                </button>
              </>
            )}
            {payloads.length > 0 && (
              <button
                type="button"
                onClick={() => setActiveTab('a2ui')}
                style={{
                  padding: '4px 10px',
                  fontSize: '0.68rem',
                  borderRadius: '6px',
                  border: '1px solid rgba(255,255,255,0.12)',
                  background: activeTab === 'a2ui' ? 'rgba(16, 185, 129, 0.25)' : 'rgba(255,255,255,0.04)',
                  color: activeTab === 'a2ui' ? '#fff' : 'var(--text-2)',
                  cursor: 'pointer',
                }}
              >
                <Layers size={11} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />
                A2UI v1.0 Envelopes ({payloads.length})
              </button>
            )}
          </div>

          <pre
            style={{
              margin: 0,
              padding: '10px',
              background: 'rgba(0, 0, 0, 0.5)',
              borderRadius: '8px',
              fontSize: '0.68rem',
              fontFamily: 'var(--mono)',
              color: '#e2e8f0',
              maxHeight: '240px',
              overflowY: 'auto',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}
          >
            {activeTab === 'rpc' && xray
              ? JSON.stringify(
                  {
                    endpoint: `${xray.endpoint}/invoke`,
                    temporal_activity_attempt: xray.attempt,
                    request: xray.request,
                    response: xray.response,
                  },
                  null,
                  2
                )
              : activeTab === 'card' && xray
              ? JSON.stringify(xray.agent_card, null, 2)
              : JSON.stringify(payloads, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

export function ProtocolXRayStrip({ messages }: { messages: { data?: any }[] }) {
  const [expanded, setExpanded] = useState(false);

  const xrayTraces: TraceWithXRay[] = messages
    .filter((m) => m.data?.updateDataModel?.path?.startsWith('/trace/'))
    .map((m) => m.data.updateDataModel.value as TraceWithXRay)
    .filter((t) => Boolean(t?.xray || (t?.a2ui_payloads && t.a2ui_payloads.length > 0)));

  if (xrayTraces.length === 0) return null;

  const retriedCount = xrayTraces.filter((t) => (t.xray?.attempt || 1) > 1).length;

  return (
    <div
      className="protocol-xray-strip"
      style={{
        width: '100%',
        maxWidth: '760px',
        margin: '8px auto 14px',
        background: 'rgba(14, 14, 28, 0.8)',
        border: '1px solid rgba(124, 92, 252, 0.3)',
        borderRadius: '14px',
        padding: '10px 14px',
      }}
    >
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        style={{
          width: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          background: 'transparent',
          border: 'none',
          color: 'var(--text)',
          cursor: 'pointer',
          fontSize: '0.8rem',
          fontWeight: 600,
        }}
      >
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
          <Radio size={15} style={{ color: '#a78bfa' }} />
          <span>🔬 Protocol X-Ray Inspector</span>
          <span
            style={{
              fontSize: '0.7rem',
              padding: '2px 8px',
              borderRadius: '999px',
              background: 'rgba(124, 92, 252, 0.18)',
              color: '#c4b5fd',
            }}
          >
            {xrayTraces.length} A2A / A2UI wire {xrayTraces.length === 1 ? 'capture' : 'captures'}
          </span>
          {retriedCount > 0 && (
            <span
              style={{
                fontSize: '0.7rem',
                padding: '2px 8px',
                borderRadius: '999px',
                background: 'rgba(251, 191, 36, 0.2)',
                color: '#fde68a',
              }}
            >
              <RefreshCw size={11} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />
              {retriedCount} Temporal 503 Retry {retriedCount === 1 ? 'Recovery' : 'Recoveries'}
            </span>
          )}
        </span>
        <span style={{ color: 'var(--text-3)', fontSize: '0.74rem' }}>
          {expanded ? 'Hide Wire JSON ▲' : 'Inspect A2A + A2UI Wire JSON ▼'}
        </span>
      </button>

      {expanded && (
        <div style={{ marginTop: '10px' }}>
          {xrayTraces.map((t, idx) => (
            <ProtocolXRayCard key={idx} trace={t} defaultOpen={idx === xrayTraces.length - 1} />
          ))}
        </div>
      )}
    </div>
  );
}

export function A2AChaosToggle({
  outage,
  toggling,
  onToggle,
}: {
  outage: boolean;
  toggling: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      className={`ctrl-btn ${outage ? 'danger' : ''}`}
      onClick={onToggle}
      disabled={toggling}
      title="Toggle HTTP 503 outage on all remote A2A specialist servers (:9001-:9004) to demonstrate Temporal Activity Retry backoff"
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '6px',
        borderColor: outage ? 'rgba(251, 191, 36, 0.65)' : undefined,
        background: outage ? 'rgba(251, 191, 36, 0.16)' : undefined,
        color: outage ? '#fde68a' : undefined,
      }}
    >
      <ShieldAlert size={14} />
      {outage ? '🔥 A2A 503 Outage Active (Click to Heal)' : '⚡ Simulate A2A 503 Outage'}
    </button>
  );
}
