/**
 * Auto-demo: automated walkthrough of the Order Tracking flow.
 * Runs through place order → fast forward → disconnect → reconnect → complete
 * with timed annotations explaining what's happening.
 */
import { useState, useCallback, useRef } from 'react';

export interface AutoDemoStep {
  label: string;
  caption: string;
  action: () => void | Promise<void>;
  delayAfter: number;  // ms before next step
}

export function useAutoDemo(steps: AutoDemoStep[]) {
  const [running, setRunning] = useState(false);
  const [currentStep, setCurrentStep] = useState(-1);
  const [caption, setCaption] = useState('');
  const cancelRef = useRef(false);

  const start = useCallback(async () => {
    cancelRef.current = false;
    setRunning(true);

    for (let i = 0; i < steps.length; i++) {
      if (cancelRef.current) break;
      setCurrentStep(i);
      setCaption(steps[i].caption);

      await steps[i].action();

      // Wait
      await new Promise<void>((resolve) => {
        const timer = setTimeout(resolve, steps[i].delayAfter);
        // Check cancel periodically
        const check = setInterval(() => {
          if (cancelRef.current) {
            clearTimeout(timer);
            clearInterval(check);
            resolve();
          }
        }, 100);
      });
    }

    setRunning(false);
    setCurrentStep(-1);
    setCaption('');
  }, [steps]);

  const stop = useCallback(() => {
    cancelRef.current = true;
    setRunning(false);
    setCurrentStep(-1);
    setCaption('');
  }, []);

  return { running, currentStep, caption, start, stop };
}

export function AutoDemoOverlay({ caption, step, total, onStop }: {
  caption: string;
  step: number;
  total: number;
  onStop: () => void;
}) {
  return (
    <div className="auto-demo-bar">
      <div className="auto-demo-progress">
        <div className="auto-demo-fill" style={{ width: `${((step + 1) / total) * 100}%` }} />
      </div>
      <div className="auto-demo-content">
        <span className="auto-demo-indicator">▶ AUTO</span>
        <span className="auto-demo-caption">{caption}</span>
        <button className="auto-demo-stop" onClick={onStop}>✕ Stop</button>
      </div>
    </div>
  );
}

export function AutoDemoButton({ onClick }: { onClick: () => void }) {
  return (
    <button className="auto-demo-btn" onClick={onClick}>
      ▶ Watch Demo
    </button>
  );
}
