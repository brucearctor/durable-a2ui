import { useState, useEffect } from 'react';
import OrderTracking from './scenarios/OrderTracking';
import ApprovalWorkflow from './scenarios/ApprovalWorkflow';
import AiConcierge from './scenarios/AiConcierge';
import IncidentCommander from './scenarios/IncidentCommander';
import './App.css';

type Scenario = 'order' | 'approval' | 'ai-concierge' | 'incident-sre' | null;

function getRouteFromPath(): Scenario {
  const path = window.location.pathname.replace(/^\/+|\/+$/g, '');
  if (path === 'order') return 'order';
  if (path === 'approval') return 'approval';
  if (path === 'ai-concierge') return 'ai-concierge';
  if (path === 'incident-sre') return 'incident-sre';
  return null;
}

function navigateTo(path: string) {
  window.history.pushState({}, '', path);
  window.dispatchEvent(new PopStateEvent('popstate'));
}

const SCENARIOS: { id: Exclude<Scenario, null>; path: string; label: string; icon: string; tagline: string }[] = [
  { id: 'order', path: '/order', label: 'Order Tracking', icon: '🍕', tagline: 'Single A2UI surface, Fragile vs. Durable crash test' },
  { id: 'approval', path: '/approval', label: 'Dual-Surface Approval', icon: '📋', tagline: 'DevFest CDMX & Campus Grants — 1 workflow, 2 role views' },
  { id: 'ai-concierge', path: '/ai-concierge', label: 'Durable Eats Concierge', icon: '🤖', tagline: 'Go ADK + A2A (:9001/:9002) + Guardrail + HITL + Child WF' },
  { id: 'incident-sre', path: '/incident-sre', label: 'CloudGuard AI SRE', icon: '🚨', tagline: 'SEV-1 Canary Diagnosis & Rollback — Go ADK + A2A (:9003/:9004)' },
];

function ScenarioTabs({ active }: { active: Scenario }) {
  return (
    <div className="scenario-tabs">
      {SCENARIOS.map((s) => (
        <button
          key={s.id}
          className={`scenario-tab ${active === s.id ? 'active' : ''}`}
          onClick={() => navigateTo(s.path)}
        >
          <span className="scenario-tab-icon">{s.icon}</span>
          <span className="scenario-tab-label">{s.label}</span>
          <span className="scenario-tab-tagline">{s.tagline}</span>
        </button>
      ))}
    </div>
  );
}

export default function App() {
  const [scenario, setScenario] = useState<Scenario>(getRouteFromPath);

  useEffect(() => {
    const onNav = () => setScenario(getRouteFromPath());
    window.addEventListener('popstate', onNav);
    return () => window.removeEventListener('popstate', onNav);
  }, []);

  return (
    <div className="app a2ui-dark">
      <header className="header" onClick={() => navigateTo('/')} style={{ cursor: 'pointer' }}>
        <h1>The Agent Interoperability Stack</h1>
        <p className="subtitle">Go ADK v2.7 × A2A v2.8 × A2UI v1.0 × Temporal WorkflowStreams</p>
        <div className="tech-badges">
          <span className="tech-badge">🐹 Go 1.26</span>
          <span className="tech-badge">🧠 ADK-Go v2.7</span>
          <span className="tech-badge">🔗 A2A v2.8</span>
          <span className="tech-badge">⚡ A2UI v1.0</span>
          <span className="tech-badge">⏳ Temporal v1.50</span>
          <a
            className="tech-badge"
            href="/slides"
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            style={{ textDecoration: 'none', borderColor: 'rgba(167, 139, 250, 0.45)', color: '#c4b5fd' }}
          >
            📽️ Talk Slides
          </a>
        </div>
      </header>

      <ScenarioTabs active={scenario} />

      {scenario === 'order' && <OrderTracking />}
      {scenario === 'approval' && <ApprovalWorkflow />}
      {scenario === 'ai-concierge' && <AiConcierge />}
      {scenario === 'incident-sre' && <IncidentCommander />}
      {scenario === null && (
        <div className="landing">
          <p className="landing-prompt">Select a live demo scenario (University Talk & GDG DevFest CDMX 2026)</p>
          <div className="landing-cards">
            {SCENARIOS.map((s) => (
              <button
                key={s.id}
                className="landing-card"
                onClick={() => navigateTo(s.path)}
              >
                <span className="landing-card-icon">{s.icon}</span>
                <span className="landing-card-label">{s.label}</span>
                <span className="landing-card-tagline">{s.tagline}</span>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
