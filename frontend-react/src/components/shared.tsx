/**
 * Shared UI components used by both Order Tracking and Approval scenarios.
 * Each scenario imports what it needs — no scenario-specific logic here.
 */
import { useState, useEffect, useRef } from 'react';
import { Circle, Wifi, WifiOff, Radio, CircleDot, Puzzle, BarChart3, CircleX, CheckCircle, AlertTriangle, RefreshCw, Zap, XCircle, Mail, Eye, HelpCircle, Share2, X, Lock, HeartCrack, FastForward, Plug, Clock, RotateCw, Hourglass } from 'lucide-react';
import type { StreamStatus, MessageLogEntry } from '../hooks/useWorkflowStream';

/* ── Status Badge ────────────────────────────────────────────────── */

const STATUS_LABELS: Record<StreamStatus, string> = {
  idle: 'Idle',
  connected: 'Live',
  reconnecting: 'Reconnecting…',
  disconnected: 'Disconnected',
  done: 'Complete',
};

export function StatusBadge({ status, workflowId, clientCount, messageCount }: {
  status: StreamStatus;
  workflowId: string;
  clientCount?: number;
  messageCount?: number;
}) {
  const StatusIcon = status === 'disconnected' ? WifiOff : status === 'connected' ? Wifi : Circle;
  return (
    <div className="connection-bar">
      <div className="connection-status">
        <StatusIcon className={`dot ${status}`} size={14} aria-hidden="true" />
        {STATUS_LABELS[status]}
        <span className="workflow-id">{workflowId}</span>
      </div>
      <div className="connection-meta">
        {(messageCount ?? 0) > 0 && (
          <span className="message-count"><Mail size={12} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />{messageCount} msg{messageCount !== 1 ? 's' : ''}</span>
        )}
        {(clientCount ?? 0) > 0 && (
          <span className="client-count">
            <Eye size={12} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} />{clientCount} viewer{clientCount !== 1 ? 's' : ''}
          </span>
        )}
      </div>
    </div>
  );
}

/* ── QR Code Share ───────────────────────────────────────────────── */

export function QRShare({ workflowId, role }: { workflowId: string; role?: string }) {
  const [showQR, setShowQR] = useState(false);
  const [configUrl, setConfigUrl] = useState('');
  const [customUrl, setCustomUrl] = useState(() => {
    try {
      return localStorage.getItem('audience_public_url') || '';
    } catch {
      return '';
    }
  });
  const [editing, setEditing] = useState(false);
  const [tempUrl, setTempUrl] = useState('');
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    fetch('/api/config')
      .then(res => res.json())
      .then(data => {
        if (data && typeof data.public_url === 'string' && data.public_url.trim()) {
          setConfigUrl(data.public_url.trim());
        }
      })
      .catch(() => {});
  }, []);

  const defaultAudienceOrigin = window.location.port === '5173'
    ? window.location.origin.replace(':5173', ':5174')
    : window.location.origin;

  const activeOrigin = (customUrl.trim() || configUrl.trim() || defaultAudienceOrigin).replace(/\/$/, '');
  const roleParam = role ? `&role=${role}` : '';
  const shareUrl = `${activeOrigin}?wf=${workflowId}${roleParam}`;
  const qrUrl = `https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(shareUrl)}&bgcolor=0c0c16&color=a78bfa`;

  const isLocalhost = activeOrigin.includes('localhost') || activeOrigin.includes('127.0.0.1');

  const handleCopy = () => {
    navigator.clipboard.writeText(shareUrl);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleSaveCustom = () => {
    const trimmed = tempUrl.trim();
    if (trimmed) {
      try { localStorage.setItem('audience_public_url', trimmed); } catch {}
      setCustomUrl(trimmed);
    } else {
      try { localStorage.removeItem('audience_public_url'); } catch {}
      setCustomUrl('');
    }
    setEditing(false);
  };

  const handleReset = () => {
    try { localStorage.removeItem('audience_public_url'); } catch {}
    setCustomUrl('');
    setEditing(false);
  };

  return (
    <div className="qr-share">
      <button className="share-btn" onClick={() => setShowQR(!showQR)}>
        {showQR ? <><X size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> Close</> : <><Share2 size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> Share Live</>}
      </button>
      {showQR && (
        <div className="qr-popover">
          <img src={qrUrl} alt="QR Code" width={180} height={180} />
          <p className="qr-hint">Scan to watch / vote on mobile phone (audience kiosk)</p>
          <button className="qr-url" onClick={handleCopy} style={{ border: 'none', background: 'rgba(255,255,255,0.03)', display: 'block', width: '100%', wordBreak: 'break-all' }}>
            {shareUrl}
            <span className="copy-hint">{copied ? '✓ Copied to clipboard!' : 'click to copy'}</span>
          </button>
          {isLocalhost && !editing && (
            <div style={{ marginTop: '10px', fontSize: '0.68rem', color: '#f59e0b', background: 'rgba(245,158,11,0.08)', padding: '6px 8px', borderRadius: '6px', textAlign: 'left', lineHeight: '1.4' }}>
              ⚠️ Phones cannot open <code>localhost</code>. Run <code>./scripts/tunnel.sh 5174</code> and set the URL below or in <code>.env</code>.
            </div>
          )}
          {!editing ? (
            <div style={{ marginTop: '8px', display: 'flex', justifyContent: 'center', gap: '8px' }}>
              <button
                onClick={() => { setTempUrl(customUrl || configUrl || ''); setEditing(true); }}
                style={{ fontSize: '0.68rem', color: 'var(--accent-lt)', background: 'transparent', border: '1px solid rgba(167,139,250,0.3)', borderRadius: '4px', padding: '3px 8px', cursor: 'pointer' }}
              >
                {customUrl || configUrl ? '✏️ Edit Public URL' : '🌐 Set Public Tunnel URL'}
              </button>
              {(customUrl || configUrl) && (
                <button
                  onClick={handleReset}
                  style={{ fontSize: '0.68rem', color: 'var(--text-3)', background: 'transparent', border: '1px solid var(--border)', borderRadius: '4px', padding: '3px 6px', cursor: 'pointer' }}
                >
                  Reset
                </button>
              )}
            </div>
          ) : (
            <div style={{ marginTop: '8px', textAlign: 'left' }}>
              <input
                type="text"
                placeholder="https://...trycloudflare.com"
                value={tempUrl}
                onChange={(e) => setTempUrl(e.target.value)}
                style={{ width: '100%', fontSize: '0.72rem', padding: '5px 8px', background: 'rgba(0,0,0,0.4)', border: '1px solid var(--border-g)', borderRadius: '4px', color: '#fff', marginBottom: '6px' }}
              />
              <div style={{ display: 'flex', gap: '6px', justifyContent: 'flex-end' }}>
                <button
                  onClick={() => setEditing(false)}
                  style={{ fontSize: '0.68rem', padding: '3px 8px', background: 'transparent', border: '1px solid var(--border)', borderRadius: '4px', color: 'var(--text-3)', cursor: 'pointer' }}
                >
                  Cancel
                </button>
                <button
                  onClick={handleSaveCustom}
                  style={{ fontSize: '0.68rem', padding: '3px 8px', background: 'var(--accent)', border: 'none', borderRadius: '4px', color: '#fff', cursor: 'pointer' }}
                >
                  Save URL
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/* ── Event Log ───────────────────────────────────────────────────── */

const TYPE_ICONS: Record<string, any> = {
  createSurface: CircleDot,
  updateComponents: Puzzle,
  updateDataModel: BarChart3,
  deleteSurface: CircleX,
};

export function EventLog({ messages, expanded, onToggle, label }: {
  messages: MessageLogEntry[];
  expanded: string | null;
  onToggle: (id: string) => void;
  label?: string;
}) {
  const bottomRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    if (autoScroll) bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages.length, autoScroll]);

  return (
    <div className="event-log">
      <div className="event-log-header">
        <span><Radio size={14} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '6px' }} />{label || 'A2UI Protocol'}</span>
        <span className="event-count">{messages.length}</span>
      </div>
      <div
        className="event-log-entries"
        onScroll={(e) => {
          const el = e.currentTarget;
          setAutoScroll(el.scrollHeight - el.scrollTop - el.clientHeight < 40);
        }}
      >
        {messages.map((msg, i) => (
          <button
            key={msg.id}
            className={`event-entry ${i === messages.length - 1 ? 'flash' : ''}`}
            onClick={() => onToggle(msg.id)}
            style={{ display: 'block', width: '100%', textAlign: 'left', border: 'none', background: 'transparent' }}
          >
            <div className="event-entry-header">
              <span className="event-icon">
                {TYPE_ICONS[msg.type] ? (
                  (() => {
                    const Icon = TYPE_ICONS[msg.type];
                    return <Icon size={12} style={{ display: 'block' }} />;
                  })()
                ) : (
                  <HelpCircle size={12} style={{ display: 'block' }} />
                )}
              </span>
              <span className="event-type">{msg.type}</span>
              <span className="event-summary">{msg.summary}</span>
              <span className="event-id">#{msg.id}</span>
            </div>
            {expanded === msg.id && (
              <pre className="event-json">{JSON.stringify(msg.data, null, 2)}</pre>
            )}
          </button>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}

/* ── Fragile Mode Overlay ────────────────────────────────────────── */

export function FragileOverlay({ onSwitchBack }: { onSwitchBack: () => void }) {
  return (
    <div className="fragile-overlay fragile-glitch-wrapper">
      <div className="fragile-content">
        <div className="fragile-icon"><Zap size={48} style={{ color: 'var(--danger)' }} /></div>
        <h2>Connection Lost</h2>
        <p>Your UI state has been destroyed.<br />This is what happens without durable workflows.</p>
        <div className="fragile-details">
          <div className="fragile-lost-item"><XCircle size={14} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '6px' }} />Component tree — gone</div>
          <div className="fragile-lost-item"><XCircle size={14} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '6px' }} />Order status — unknown</div>
          <div className="fragile-lost-item"><XCircle size={14} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '6px' }} />User actions — lost</div>
        </div>
        <button className="place-order-btn" onClick={onSwitchBack}>
          <CheckCircle size={18} /> Switch to Durable Mode
        </button>
        <p className="fragile-footnote">
          In Durable Mode, the UI rebuilds from Temporal workflow state.
        </p>
      </div>
    </div>
  );
}

/* ── Controls Bar ────────────────────────────────────────────────── */

export function ControlsBar({
  status,
  fragileMode,
  devMode,
  onFragileToggle,
  onDevToggle,
  onFastForward,
  onDisconnect,
  onReset,
}: {
  status: StreamStatus;
  fragileMode: boolean;
  devMode: boolean;
  onFragileToggle: (fragile: boolean) => void;
  onDevToggle: () => void;
  onFastForward: () => void;
  onDisconnect: () => void;
  onReset: () => void;
}) {
  return (
    <div className="controls">
      <div className="mode-toggle">
        <button
          className={`mode-btn ${!fragileMode ? 'active' : ''}`}
          onClick={() => onFragileToggle(false)}
        >
          <Lock size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> Durable
        </button>
        <button
          className={`mode-btn fragile-mode ${fragileMode ? 'active' : ''}`}
          onClick={() => onFragileToggle(true)}
        >
          <HeartCrack size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> Fragile
        </button>
      </div>

      {status !== 'done' && (
        <>
          <button className="ctrl-btn" onClick={onFastForward}>
            <FastForward size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> Fast Forward
          </button>
          <button
            className="ctrl-btn disconnect"
            onClick={onDisconnect}
            disabled={status === 'disconnected' || status === 'reconnecting'}
          >
            {status === 'disconnected' ? <><Hourglass size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> Recovering...</> :
             status === 'reconnecting' ? <><RefreshCw size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> Reconnecting...</> :
             <><Plug size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> Disconnect</>}
          </button>
        </>
      )}
      {status === 'done' && (
        <button className="ctrl-btn primary" onClick={onReset}><RotateCw size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> New</button>
      )}
      <a
        className="ctrl-btn temporal-link"
        href="http://localhost:8233"
        target="_blank"
        rel="noopener noreferrer"
      >
        <Clock size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> Temporal UI
      </a>
      <button
        className={`ctrl-btn devtools ${devMode ? 'active' : ''}`}
        onClick={onDevToggle}
      >
        {'</>'} Dev
      </button>
    </div>
  );
}

/* ── Connection Banner ──────────────────────────────────────────── */

export function ConnectionBanner({ status, messageCount }: { status: StreamStatus; messageCount?: number }) {
  const [showReconnected, setShowReconnected] = useState(false);
  const prevStatus = useRef(status);

  // Show "Reconnected!" celebration briefly after recovering
  useEffect(() => {
    if (prevStatus.current === 'reconnecting' && status === 'connected') {
      setShowReconnected(true);
      const timer = setTimeout(() => setShowReconnected(false), 3000);
      return () => clearTimeout(timer);
    }
    prevStatus.current = status;
  }, [status]);

  if (showReconnected) {
    return (
      <div className="connection-banner banner-reconnected">
        <CheckCircle size={16} />
        <span>
          Reconnected — replayed {messageCount ?? 0} events.
          <strong> Zero data loss.</strong>
        </span>
      </div>
    );
  }

  if (status === 'connected' || status === 'idle') return null;

  const config: Record<string, { className: string; icon: any; text: string }> = {
    disconnected: {
      className: 'banner-disconnected',
      icon: AlertTriangle,
      text: 'Worker disconnected — workflow continues on Temporal. Waiting for restart…',
    },
    reconnecting: {
      className: 'banner-reconnecting',
      icon: RefreshCw,
      text: `Reconnecting — replaying ${messageCount ?? 0} events from stream…`,
    },
    done: {
      className: 'banner-done',
      icon: CheckCircle,
      text: 'Workflow complete. All state is persisted in Temporal history.',
    },
  };

  const c = config[status];
  if (!c) return null;

  const BannerIcon = c.icon;

  return (
    <div className={`connection-banner ${c.className}`}>
      <BannerIcon size={16} />
      <span>{c.text}</span>
    </div>
  );
}
