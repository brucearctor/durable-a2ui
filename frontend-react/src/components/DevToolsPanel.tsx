import { useState, useEffect, useRef } from 'react';
import { Terminal, X, ChevronRight, ChevronDown, Activity, List, PenTool as Tool, MessageSquare, Layers } from 'lucide-react';
import { ProtocolXRayCard } from './ProtocolXRay';
import './DevToolsPanel.css';

interface DevToolsPanelProps {
  messages: any[];
  onClose: () => void;
}

export default function DevToolsPanel({ messages, onClose }: DevToolsPanelProps) {
  const [expandedEvents, setExpandedEvents] = useState<Set<number>>(new Set());
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const traces = messages
    .filter(m => m.data?.updateDataModel?.path?.startsWith('/trace/'))
    .map(m => m.data.updateDataModel.value);

  const metrics = messages
    .filter(m => m.data?.updateDataModel?.path === '/metrics')
    .map(m => m.data.updateDataModel.value)
    .pop();

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [traces.length]);

  const toggleExpand = (index: number) => {
    setExpandedEvents(prev => {
      const next = new Set(prev);
      if (next.has(index)) {
        next.delete(index);
      } else {
        next.add(index);
      }
      return next;
    });
  };

  const getBadgeColor = (type: string) => {
    switch (type) {
      case 'turn_start':
      case 'a2a_fanout_start': return 'blue';
      case 'agent_handoff':
      case 'schema_migration': return 'purple';
      case 'tool_call': return 'green';
      case 'function_response':
      case 'a2a_specialist_done': return 'teal';
      case 'turn_end': return 'gray';
      default: return 'gray';
    }
  };

  const getEventIcon = (type: string) => {
    switch (type) {
      case 'turn_start': return <MessageSquare size={12} />;
      case 'agent_handoff': return <Activity size={12} />;
      case 'schema_migration': return <Layers size={12} />;
      case 'tool_call': return <Tool size={12} />;
      case 'function_response':
      case 'a2a_specialist_done': return <Terminal size={12} />;
      case 'turn_end': return <List size={12} />;
      default: return <Activity size={12} />;
    }
  };

  const getEventSummary = (trace: any) => {
    if (trace.type === 'turn_start' && trace.message) {
      return trace.message.length > 30 ? trace.message.substring(0, 30) + '...' : trace.message;
    }
    if (trace.type === 'agent_handoff' && trace.agent) {
      return trace.agent;
    }
    if (trace.type === 'schema_migration') {
      return `${trace.surface || 'surface'} \u2192 v${trace.version || 2}`;
    }
    if (trace.type === 'a2a_specialist_done') {
      const attempt = trace.xray?.attempt || 1;
      return `${trace.specialist || 'specialist'} \u2192 ${trace.status || 'ok'}${attempt > 1 ? ` (Attempt #${attempt})` : ''}`;
    }
    if (trace.type === 'tool_call' && trace.tool) {
      return trace.tool;
    }
    if (trace.type === 'function_response') {
      const attempt = trace.xray?.attempt || 1;
      return `${trace.function || 'unknown'} \u2192 ${trace.status || 'ok'}${attempt > 1 ? ` (Attempt #${attempt})` : ''}`;
    }
    if (trace.type === 'turn_end' && trace.tokens !== undefined) {
      return `${trace.tokens} tokens`;
    }
    return trace.type;
  };

  const formatTokens = (tokens?: number) => {
    if (tokens === undefined) return '0';
    if (tokens >= 1000) return (tokens / 1000).toFixed(1) + 'K';
    return tokens.toString();
  };

  const formatCost = (cost?: number) => {
    if (cost === undefined) return '$0.0000';
    return '$' + cost.toFixed(4);
  };

  const formatTimeAgo = (ts?: number) => {
    if (!ts) return 'just now';
    const seconds = Math.floor((Date.now() - ts) / 1000);
    if (seconds < 2) return 'just now';
    if (seconds < 60) return `${seconds}s ago`;
    return `${Math.floor(seconds / 60)}m ago`;
  };

  return (
    <div className="devtools-panel">
      <div className="devtools-header">
        <div className="devtools-title">
          <Terminal size={16} />
          <span>Dev Mode</span>
        </div>
        <button className="devtools-close" onClick={onClose}>
          <X size={16} />
        </button>
      </div>

      {/* Live data flow indicator */}
      <div className="devtools-flow">
        <span className="flow-label">Live event stream</span>
      </div>

      <div className="devtools-metrics">
        <div className="metric-card tokens">
          <div className="metric-label">Tokens</div>
          <div className="metric-value">{formatTokens(metrics?.total_tokens)}</div>
        </div>
        <div className="metric-card cost">
          <div className="metric-label">Cost</div>
          <div className="metric-value">{formatCost(metrics?.total_cost_usd)}</div>
        </div>
        <div className="metric-card turns">
          <div className="metric-label">Turns</div>
          <div className="metric-value">{metrics?.turn_count || 0}</div>
        </div>
        <div className="metric-card tools">
          <div className="metric-label">Tools Called</div>
          <div className="metric-value">{metrics?.tool_call_count || 0}</div>
        </div>
      </div>

      <div className="devtools-timeline">
        {traces.length === 0 && (
          <div className="devtools-empty">No trace events yet...</div>
        )}
        {traces.map((trace, i) => {
          const isExpanded = expandedEvents.has(i);
          return (
            <div key={i} className="trace-event">
              <div className="trace-event-header" onClick={() => toggleExpand(i)}>
                <div className={`trace-badge ${getBadgeColor(trace.type)}`}>
                  {getEventIcon(trace.type)}
                  <span>{trace.type}</span>
                </div>
                <div className="trace-summary">{getEventSummary(trace)}</div>
                <div className="trace-time">{formatTimeAgo(trace.timestamp)}</div>
                <div className="trace-expand-icon">
                  {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </div>
              </div>
              {isExpanded && (
                <div className="trace-detail">
                  <ProtocolXRayCard trace={trace} defaultOpen={true} />
                  <pre>{JSON.stringify(trace, null, 2)}</pre>
                </div>
              )}
            </div>
          );
        })}
        <div ref={messagesEndRef} />
      </div>
    </div>
  );
}
