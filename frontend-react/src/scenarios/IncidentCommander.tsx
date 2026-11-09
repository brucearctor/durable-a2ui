import { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { A2uiSurface } from '@a2ui/react';
import { ShieldAlert, Activity, ShieldCheck, Rocket, CheckCircle, ShieldX, Send, FastForward, RefreshCw, Loader, Zap } from 'lucide-react';
import { useWorkflowStream } from '../hooks/useWorkflowStream';
import type { StreamOptions } from '../hooks/useWorkflowStream';
import { StatusBadge, ConnectionBanner } from '../components/shared';
import { useWorkerStatus, WorkerKillButton, WorkerDeadOverlay } from '../components/WorkerStatus';
import DevToolsPanel from '../components/DevToolsPanel';

const SRE_PROMPTS = [
  { icon: <Activity size={16} />, text: 'Investigate SEV-1 telemetry and canary diff for checkout-api' },
  { icon: <ShieldCheck size={16} />, text: 'Evaluate blast radius and SecOps policy' },
  { icon: <Zap size={16} />, text: 'Execute canary rollback RB-402 now!' },
];

export default function IncidentCommander() {
  const [workflowId, setWorkflowId] = useState<string | null>(null);
  const [devMode, setDevMode] = useState(false);
  const [loading, setLoading] = useState(false);
  const [inputText, setInputText] = useState('');
  const [commanderName, setCommanderName] = useState('On-Call SRE Lead (CDMX / Campus)');
  const [serviceName, setServiceName] = useState('checkout-api (mx-central1 & us-central1 GKE)');
  const [error, setError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const streamOptions: StreamOptions | undefined = useMemo(() => {
    if (!workflowId) return undefined;
    return {
      streamUrl: `/api/incidents/${workflowId}/stream`,
      signalUrl: `/api/incidents/${workflowId}/signal`,
      onAction: (eventName: string) => {
        if (!workflowId) return false;
        fetch(`/api/incidents/${workflowId}/signal/user_action`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ event: eventName }),
        }).catch((err) => console.error('Failed to send SRE action:', err));
        return true;
      },
    };
  }, [workflowId]);

  const { surfaces, status, sendSignal, messages } = useWorkflowStream(workflowId, streamOptions);
  const { workerAlive, killing, restarting, killWorker, restartWorker, downSince } = useWorkerStatus();

  const workflowIdRef = useRef(workflowId);
  workflowIdRef.current = workflowId;

  const sendSignalDirect = useCallback(async (name: string) => {
    const wfId = workflowIdRef.current;
    if (!wfId) return;
    await fetch(`/api/incidents/${wfId}/signal/${name}`, { method: 'POST' });
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
      'Telemetry Investigator': 'telemetry_investigator',
      'Mitigation Planner': 'mitigation_planner',
      'Remediation Executor': 'remediation_executor',
    };
    for (let i = chatMessages.length - 1; i >= 0; i--) {
      const msg = chatMessages[i];
      if (msg.role === 'system' && msg.text.includes('Transferring to')) {
        for (const [label, id] of Object.entries(agentMap)) {
          if (msg.text.includes(label)) return id;
        }
      }
    }
    return 'incident_commander';
  }, [chatMessages]);

  const completedAgents = useMemo(() => {
    const agents = ['incident_commander', 'telemetry_investigator', 'mitigation_planner', 'remediation_executor'];
    const activeIdx = agents.indexOf(activeAgent);
    if (activeIdx === -1) return [];
    if (status === 'done') return agents;
    return agents.slice(0, activeIdx);
  }, [activeAgent, status]);

  const approvalState = useMemo(() => {
    let latest: { status: string; commander?: string; service?: string; runbook?: string; blast_radius?: string; slo_burn_rate?: string } | null = null;
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
      await fetch(`/api/incidents/${workflowId}/approve`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ decision }),
      });
    } catch (err) {
      console.error('Approval failed:', err);
      setError('Failed to send commander decision.');
    }
  }, [workflowId]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages.length, isTyping, surfaces.length, status]);

  const startIncident = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/incidents', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ commander_name: commanderName, service_name: serviceName }),
      });
      const result = await res.json();
      setWorkflowId(result.workflow_id);
    } catch (err) {
      console.error('Failed to start incident commander:', err);
      setError('Could not start Incident Commander. Is the Go backend running?');
    } finally {
      setLoading(false);
    }
  }, [commanderName, serviceName]);

  const sendMessage = useCallback(async () => {
    if (!inputText.trim() || !workflowId) return;
    const text = inputText.trim();
    setInputText('');
    try {
      await fetch(`/api/incidents/${workflowId}/message`, {
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
          <div className="chat-start-icon"><ShieldAlert size={48} /></div>
          <div className="chat-start-title">CloudGuard AI SRE Incident Commander</div>
          <div className="chat-start-subtitle">
            Durable Multi-Agent Incident Response — Go ADK v2.7 + A2A v2.8 (:9003 Telemetry / :9004 SecOps) + A2UI v1.0 + Temporal
          </div>
        </div>
        <div className="order-form">
          <h2><ShieldAlert size={20} style={{ display: 'inline', verticalAlign: 'text-bottom', marginRight: '8px' }} />Launch SEV-1 Incident War Room</h2>
          <div className="form-field">
            <label htmlFor="sre-commander">Incident Commander</label>
            <input
              id="sre-commander"
              type="text"
              value={commanderName}
              onChange={(e) => setCommanderName(e.target.value)}
            />
          </div>
          <div className="form-field">
            <label htmlFor="sre-service">Impacted Service & Region</label>
            <input
              id="sre-service"
              type="text"
              value={serviceName}
              onChange={(e) => setServiceName(e.target.value)}
            />
          </div>
          <button
            className="place-order-btn"
            onClick={startIncident}
            disabled={loading || !commanderName.trim()}
          >
            {loading ? (
              <><Loader size={18} className="spin" style={{ marginRight: '6px' }} /> Initializing War Room...</>
            ) : (
              <><Rocket size={18} style={{ marginRight: '6px' }} /> Start SEV-1 Response</>
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
            title="Fast forward next SRE step (F)"
          >
            <FastForward size={14} style={{ display: 'inline', verticalAlign: 'text-bottom' }} /> Fast Forward Step
          </button>
          <WorkerKillButton onKill={killWorker} killing={killing} workerAlive={workerAlive} />
          <button className="ctrl-btn" onClick={resetDemo} title="New incident (N)">
            <RefreshCw size={14} style={{ display: 'inline', verticalAlign: 'text-bottom' }} /> New Incident
          </button>
          <button className={`ctrl-btn devtools ${devMode ? 'active' : ''}`} onClick={() => setDevMode(!devMode)} title="Dev Mode (D)">
            {'</>'} Dev
          </button>
        </div>

        <ConnectionBanner status={status} messageCount={messages.length} />

        {approvalState?.status === 'pending' && (
          <div className="approval-banner">
            <div className="approval-banner-header">
              <ShieldCheck size={20} />
              <strong>Incident Commander HITL Approval Required (Change-Freeze Gate)</strong>
            </div>
            <div className="approval-banner-body">
              <p>
                <strong>Runbook:</strong> {approvalState.runbook}
              </p>
              <p>
                <strong>Blast Radius:</strong> {approvalState.blast_radius} | <strong>SLO Burn:</strong> {approvalState.slo_burn_rate}
              </p>
            </div>
            <div className="approval-banner-actions">
              <button className="approve-btn" onClick={() => handleApproval('approved')}>
                <CheckCircle size={16} /> Authorize Rollback (RB-402)
              </button>
              <button className="reject-btn" onClick={() => handleApproval('rejected')}>
                <ShieldX size={16} /> Abort Rollback
              </button>
            </div>
          </div>
        )}
        {approvalState?.status === 'approved' && (
          <div className="approval-banner resolved approved">
            <CheckCircle size={18} /> Runbook RB-402 authorized by Incident Commander
          </div>
        )}
        {approvalState?.status === 'rejected' && (
          <div className="approval-banner resolved rejected">
            <ShieldX size={18} /> Rollback aborted by Incident Commander
          </div>
        )}

        <div className="agent-pipeline">
          {[
            { id: 'incident_commander', icon: <ShieldAlert />, label: 'Commander' },
            { id: 'telemetry_investigator', icon: <Activity />, label: 'Telemetry (:9003)' },
            { id: 'mitigation_planner', icon: <ShieldCheck />, label: 'SecOps (:9004)' },
            { id: 'remediation_executor', icon: <Rocket />, label: 'Rollback RB-402' },
          ].map((agent) => {
            const isCompleted = completedAgents.includes(agent.id);
            const isActive = activeAgent === agent.id;
            return (
              <div key={agent.id} className={`agent-pipeline-item ${isCompleted ? 'completed' : ''}`}>
                <div className={`agent-node ${isActive ? 'active' : ''} ${isCompleted ? 'completed' : ''}`}>
                  {agent.icon}
                  {isCompleted && <span className="completed-badge"><CheckCircle /></span>}
                </div>
                <span className="agent-node-label">{agent.label}</span>
              </div>
            );
          })}
        </div>

        <div className="chat-layout">
          <div className="chat-messages">
            {chatMessages.map((msg) => (
              <div key={msg.id} className={`chat-bubble ${msg.role}${msg.streaming ? ' streaming' : ''}`}>
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

            <div className="prompt-chips">
              {SRE_PROMPTS.map((chip) => (
                <button
                  key={chip.text}
                  className="prompt-chip"
                  onClick={() => setInputText(chip.text)}
                >
                  {chip.icon} <span style={{ marginLeft: '4px' }}>{chip.text}</span>
                </button>
              ))}
            </div>

            {surfaces.filter(s => s.id !== 'chat').map((surface) => (
              <div key={surface.id} className="chat-a2ui-surface">
                <A2uiSurface surface={surface} />
              </div>
            ))}

            {isTyping && (
              <div className="chat-typing" role="status">
                CloudGuard SRE agent is analyzing…
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
            <input
              className="chat-input"
              type="text"
              placeholder="Ask CloudGuard SRE Commander or click a prompt above…"
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
