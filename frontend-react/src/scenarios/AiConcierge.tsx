import { useState, useCallback, useEffect, useRef, useMemo, Component, type ReactNode, type ErrorInfo } from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { A2uiSurface } from '@a2ui/react';
import { Bot, ClipboardList, ShoppingCart, CheckCircle, Mic, Square, Send, FastForward, RefreshCw, Pizza, Zap, CakeSlice, MessageCircle, Loader, Rocket, AlertTriangle, ShieldCheck, ShieldX } from 'lucide-react';
import { useWorkflowStream } from '../hooks/useWorkflowStream';
import type { StreamOptions } from '../hooks/useWorkflowStream';
import { StatusBadge, ConnectionBanner, QRShare } from '../components/shared';
import { useWorkerStatus, WorkerKillButton, WorkerDeadOverlay } from '../components/WorkerStatus';
import { A2AChaosToggle, ProtocolXRayStrip } from '../components/ProtocolXRay';
import DevToolsPanel from '../components/DevToolsPanel';

class A2uiErrorBoundary extends Component<{ children: ReactNode }, { hasError: boolean; error?: Error }> {
  state = { hasError: false, error: undefined as Error | undefined };

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('A2UI surface render error:', error, info);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="a2ui-error-fallback">
          <span><AlertTriangle size={14} aria-hidden="true" style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '4px' }} /> UI component failed to render</span>
          <button onClick={() => this.setState({ hasError: false })}>Retry</button>
        </div>
      );
    }
    return this.props.children;
  }
}

const PROMPT_CHIPS = [
  { icon: <Pizza size={16} />, text: 'Show me the menu' },
  { icon: <Zap size={16} />, text: 'Order 4 Tacos al Pastor and 2 Horchata Artesanal' },
  { icon: <CakeSlice size={16} />, text: 'Add 2 Brown Sugar Boba and 2 Margherita Pizzas' },
  { icon: <ShoppingCart size={16} />, text: 'Team Feast: 10 Tacos al Pastor' },
];

export default function AiConcierge() {
  const [workflowId, setWorkflowId] = useState<string | null>(null);
  const [devMode, setDevMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [inputText, setInputText] = useState('');
  const [customerName, setCustomerName] = useState('Alex Rivera');
  const [error, setError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (error) {
      const t = setTimeout(() => setError(null), 5000);
      return () => clearTimeout(t);
    }
  }, [error]);

  const streamOptions: StreamOptions | undefined = useMemo(() => {
    if (!workflowId) return undefined;
    return {
      streamUrl: `/api/concierge/${workflowId}/stream`,
      signalUrl: `/api/concierge/${workflowId}/signal`,
      onAction: (eventName: string, _action: any) => {
        if (!workflowId) return false;

        if (eventName.startsWith('track_order:')) {
          const orderWfId = eventName.split(':')[1];
          if (orderWfId) {
            window.open(`/order?wf=${orderWfId}`, '_blank');
            return true;
          }
        }

        fetch(`/api/concierge/${workflowId}/signal/user_action`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ event: eventName }),
        }).catch((err) => console.error('Failed to send action:', err));
        return true;
      },
    };
  }, [workflowId]);

  const { surfaces, status, sendSignal, messages } =
    useWorkflowStream(workflowId, streamOptions);

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

  const workflowIdRef = useRef(workflowId);
  workflowIdRef.current = workflowId;

  const sendSignalDirect = useCallback(async (name: string) => {
    const wfId = workflowIdRef.current;
    if (!wfId) return;
    await fetch(`/api/concierge/${wfId}/signal/${name}`, { method: 'POST' });
  }, []);

  const chatMessages = useMemo(() => {
    const msgMap = new Map<string, { role: string; text: string; id: string; streaming?: boolean }>();
    for (const entry of messages) {
      if (entry.type === 'updateDataModel') {
        const raw = entry.data as any;
        const udm = raw?.updateDataModel;
        if (udm?.path?.startsWith('/messages/')) {
          const val = udm.value;
          if (val?.role && val?.text) {
            msgMap.set(val.id || udm.path, {
              role: val.role,
              text: val.text,
              id: val.id || udm.path,
              streaming: val.streaming,
            });
          }
        }
      }
    }
    return Array.from(msgMap.values());
  }, [messages]);

  const isTyping = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const raw = messages[i].data as any;
      const udm = raw?.updateDataModel;
      if (udm?.path === '/typing') return udm.value === true;
    }
    return false;
  }, [messages]);

  const activeAgent = useMemo(() => {
    const agentMap: Record<string, string> = {
      'Menu Advisor': 'menu_advisor',
      'Cart Manager': 'cart_manager',
      'Order Confirmer': 'order_confirmer',
    };
    for (let i = chatMessages.length - 1; i >= 0; i--) {
      const msg = chatMessages[i];
      if (msg.role === 'system' && msg.text.includes('Transferring to')) {
        for (const [label, id] of Object.entries(agentMap)) {
          if (msg.text.includes(label)) return id;
        }
      }
    }
    return 'concierge';
  }, [chatMessages]);

  const completedAgents = useMemo(() => {
    const agents = ['concierge', 'menu_advisor', 'cart_manager', 'order_confirmer'];
    const activeIdx = agents.indexOf(activeAgent);
    if (activeIdx === -1) return [];
    if (status === 'done') return agents;
    return agents.slice(0, activeIdx);
  }, [activeAgent, status]);

  const approvalState = useMemo(() => {
    let latest: { status: string; total?: number; threshold?: number; items?: string; customer?: string } | null = null;
    for (const entry of messages) {
      if (entry.type === 'updateDataModel') {
        const raw = entry.data as any;
        const udm = raw?.updateDataModel;
        if (udm?.path === '/approval' && udm?.value) {
          latest = udm.value;
        }
      }
    }
    return latest;
  }, [messages]);

  const handleApproval = useCallback(async (decision: 'approved' | 'rejected') => {
    if (!workflowId) return;
    try {
      await fetch(`/api/concierge/${workflowId}/approve`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ decision }),
      });
    } catch (err) {
      console.error('Approval failed:', err);
      setError('Failed to send approval decision.');
    }
  }, [workflowId]);

  const [isListening, setIsListening] = useState(false);
  const recognitionRef = useRef<any>(null);

  const toggleVoice = useCallback(() => {
    const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (!SpeechRecognition) return;

    if (isListening && recognitionRef.current) {
      recognitionRef.current.stop();
      setIsListening(false);
      return;
    }

    const recognition = new SpeechRecognition();
    recognition.continuous = false;
    recognition.interimResults = false;
    recognition.lang = 'en-US';
    recognition.onresult = (e: any) => {
      const transcript = e.results[0][0].transcript;
      setInputText(transcript);
      setIsListening(false);
    };
    recognition.onerror = () => setIsListening(false);
    recognition.onend = () => setIsListening(false);
    recognitionRef.current = recognition;
    recognition.start();
    setIsListening(true);
  }, [isListening]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages.length, isTyping, surfaces.length, status]);

  const startChat = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/concierge', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ customer_name: customerName }),
      });
      const result = await res.json();
      setWorkflowId(result.workflow_id);
    } catch (err) {
      console.error('Failed to start concierge:', err);
      setError('Could not start chat. Is the Go backend running?');
    } finally {
      setLoading(false);
    }
  }, [customerName]);

  const sendMessage = useCallback(async () => {
    if (!inputText.trim() || !workflowId) return;
    const text = inputText.trim();
    setInputText('');
    try {
      await fetch(`/api/concierge/${workflowId}/message`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text }),
      });
    } catch (err) {
      console.error('Failed to send message:', err);
      setError('Message failed to send.');
    }
  }, [inputText, workflowId]);

  const resetDemo = useCallback(() => {
    setWorkflowId(null);
    setInputText('');
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;

      switch (e.key.toLowerCase()) {
        case 'f': sendSignalDirect('fast_forward'); break;
        case 'n': resetDemo(); break;
        case 'd': setDevMode(prev => !prev); break;
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [sendSignalDirect, resetDemo]);

  if (!workflowId) {
    return (
      <div className="start-section">
        <div className="chat-start">
          <div className="chat-start-icon"><Bot size={48} /></div>
          <div className="chat-start-title">Durable Eats AI Concierge</div>
          <div className="chat-start-subtitle">
            Go ADK v2.7 Multi-Agent + A2A v2.8 (:9001 / :9002) + A2UI v1.0 + Temporal
          </div>
        </div>
        <div className="order-form">
          <h2><MessageCircle size={20} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '8px' }} />Start a Conversation</h2>
          <div className="form-field">
            <label htmlFor="concierge-name">Your Name</label>
            <input
              id="concierge-name"
              type="text"
              value={customerName}
              onChange={(e) => setCustomerName(e.target.value)}
              placeholder="Enter your name"
            />
          </div>
          <button
            className="place-order-btn"
            onClick={startChat}
            disabled={loading || !customerName.trim()}
          >
            {loading ? (
              <><Loader size={18} className="spin" style={{ marginRight: '6px' }} /> Starting...</>
            ) : (
              <><Rocket size={18} style={{ marginRight: '6px' }} /> Start Chat</>
            )}
          </button>
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
        <StatusBadge status={status} workflowId={workflowId} messageCount={messages.length} />

        <div className="controls">
          <button
            className="ctrl-btn"
            onClick={() => sendSignal('fast_forward')}
            disabled={status !== 'connected'}
            title="Fast forward (F)"
          >
            <FastForward size={14} style={{ display: 'inline', verticalAlign: 'text-bottom' }} /> Fast Forward
          </button>
          <A2AChaosToggle outage={a2aOutage} toggling={togglingChaos} onToggle={toggleA2AChaos} />
          <WorkerKillButton onKill={killWorker} killing={killing} workerAlive={workerAlive} />
          <QRShare workflowId={workflowId} />
          <button className="ctrl-btn" onClick={resetDemo} title="New session (N)">
            <RefreshCw size={14} style={{ display: 'inline', verticalAlign: 'text-bottom' }} /> New Chat
          </button>
          <button className={`ctrl-btn devtools ${devMode ? 'active' : ''}`} onClick={() => setDevMode(!devMode)} title="Dev Mode (D)">
            {'</>'} Dev
          </button>
        </div>

        <ConnectionBanner status={status} messageCount={messages.length} />
        <ProtocolXRayStrip messages={messages} />

        {approvalState?.status === 'pending' && (
          <div className="approval-banner">
            <div className="approval-banner-header">
              <ShieldCheck size={20} />
              <strong>Manager Approval Required (HITL Gate)</strong>
            </div>
            <div className="approval-banner-body">
              <p>
                Order for <strong>{approvalState.customer}</strong>: {approvalState.items}
              </p>
              <p className="approval-total">
                Total: <strong>${approvalState.total?.toFixed(2)}</strong>
                <span className="approval-threshold">
                  {' '}(exceeds ${approvalState.threshold?.toFixed(2)} limit)
                </span>
              </p>
            </div>
            <div className="approval-banner-actions">
              <button
                className="approve-btn"
                onClick={() => handleApproval('approved')}
              >
                <CheckCircle size={16} /> Approve
              </button>
              <button
                className="reject-btn"
                onClick={() => handleApproval('rejected')}
              >
                <ShieldX size={16} /> Reject
              </button>
            </div>
          </div>
        )}
        {approvalState?.status === 'approved' && (
          <div className="approval-banner resolved approved">
            <CheckCircle size={18} /> Order approved by manager
          </div>
        )}
        {approvalState?.status === 'rejected' && (
          <div className="approval-banner resolved rejected">
            <ShieldX size={18} /> Order rejected by manager
          </div>
        )}

        <div className="agent-pipeline">
          {[
            { id: 'concierge', icon: <Bot />, label: 'Concierge' },
            { id: 'menu_advisor', icon: <ClipboardList />, label: 'Menu + A2A (:9001)' },
            { id: 'cart_manager', icon: <ShoppingCart />, label: 'Cart + A2A (:9002)' },
            { id: 'order_confirmer', icon: <CheckCircle />, label: 'Confirm + Child WF' },
          ].map((agent) => {
            const isCompleted = completedAgents.includes(agent.id);
            const isActive = activeAgent === agent.id;
            return (
              <div key={agent.id} className={`agent-pipeline-item ${isCompleted ? 'completed' : ''}`}>
                <div className={`agent-node ${isActive ? 'active' : ''} ${isCompleted ? 'completed' : ''}`}>
                  {agent.icon}
                  {isCompleted && (
                    <span className="completed-badge"><CheckCircle /></span>
                  )}
                </div>
                <span className="agent-node-label">{agent.label}</span>
              </div>
            );
          })}
        </div>

        <div className="sr-only" role="log">
          {chatMessages.length > 0 ? chatMessages[chatMessages.length - 1].text : ''}
        </div>

        <div className="chat-layout">
          <div className="chat-messages">
            {chatMessages.map((msg) => (
              <div
                key={msg.id}
                className={`chat-bubble ${msg.role}${msg.streaming ? ' streaming' : ''}`}
              >
                {msg.role === 'assistant' ? (
                  <>
                    <Markdown remarkPlugins={[remarkGfm]}>{msg.text}</Markdown>
                    {msg.streaming && <span className="stream-cursor">▊</span>}
                  </>
                ) : (
                  msg.text
                )}
              </div>
            ))}

            {chatMessages.length <= 2 && (
              <div className="prompt-chips">
                {PROMPT_CHIPS.map((chip) => (
                  <button
                    key={chip.text}
                    className="prompt-chip"
                    onClick={() => {
                      setInputText(chip.text);
                    }}
                  >
                    {chip.icon} <span style={{ marginLeft: '4px' }}>{chip.text}</span>
                  </button>
                ))}
              </div>
            )}

            {surfaces.filter(s => s.id !== 'chat').map((surface) => (
              <div key={surface.id} className="chat-a2ui-surface">
                <A2uiErrorBoundary>
                  <A2uiSurface surface={surface} />
                </A2uiErrorBoundary>
              </div>
            ))}

            {isTyping && (
              <div className="chat-typing" role="status">
                AI is thinking…
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>

          {error && (
            <div className="error-toast" role="alert">
              {error}
              <button onClick={() => setError(null)} aria-label="Dismiss error">×</button>
            </div>
          )}

          <div className="chat-input-bar">
            <button
              className={`voice-btn ${isListening ? 'listening' : ''}`}
              onClick={toggleVoice}
              disabled={status !== 'connected'}
              title="Voice input"
              aria-label="Voice input"
            >
              {isListening ? <Square size={16} /> : <Mic size={16} />}
            </button>
            <input
              className="chat-input"
              type="text"
              placeholder={isListening ? 'Listening…' : 'Type a message…'}
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  sendMessage();
                }
              }}
              disabled={status !== 'connected'}
            />
            <button
              className="chat-send-btn"
              onClick={sendMessage}
              disabled={!inputText.trim() || status !== 'connected'}
              aria-label="Send message"
            >
              <Send size={18} />
            </button>
          </div>
        </div>
      </div>
      {devMode && <DevToolsPanel messages={messages} onClose={() => setDevMode(false)} />}
    </div>
  );
}
