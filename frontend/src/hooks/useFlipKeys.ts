import { useEffect, useRef } from 'react';

interface FlipKeyHandlers {
  enabled: boolean;
  onPrev: () => void;
  onNext: () => void;
  /** Space: flip to the other image. Only bound while `spaceToggles` is true. */
  onToggle?: () => void;
  spaceToggles?: boolean;
}

const typing = (t: EventTarget | null) => {
  const el = t as HTMLElement | null;
  if (!el?.tagName) return false;
  return (
    el.isContentEditable ||
    ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName) ||
    el.getAttribute('role') === 'slider'
  );
};

/** ←/→ (and optionally Space) flip between images; ignored while typing or in a dialog. */
export function useFlipKeys(h: FlipKeyHandlers) {
  const latest = useRef(h);
  latest.current = h;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const cur = latest.current;
      if (!cur.enabled || e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey) return;
      if (typing(e.target)) return;
      // Don't hijack keys meant for an open dialog (e.g. the alignment dialog).
      if (document.querySelector('[role="dialog"]')) return;
      if (e.key === 'ArrowLeft') {
        e.preventDefault();
        cur.onPrev();
      } else if (e.key === 'ArrowRight') {
        e.preventDefault();
        cur.onNext();
      } else if (e.key === ' ' && cur.spaceToggles && cur.onToggle) {
        if ((e.target as HTMLElement | null)?.tagName === 'BUTTON') return;
        e.preventDefault();
        cur.onToggle();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);
}
