import { useEffect, useRef } from 'react';
import type { RefObject } from 'react';

interface FlipKeyHandlers {
  enabled: boolean;
  onPrev: () => void;
  onNext: () => void;
  /** Space: flip to the other image. Only bound while `spaceToggles` is true. */
  onToggle?: () => void;
  spaceToggles?: boolean;
  /**
   * The element these keys belong to. Keys are ignored while it is hidden
   * (e.g. a mounted but display:none mode) and, while something is fullscreen,
   * unless it is inside the fullscreen element — so only the visible viewer reacts.
   */
  scope?: RefObject<HTMLElement | null>;
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

const fullscreenEl = () =>
  document.fullscreenElement ?? (document as Document & { webkitFullscreenElement?: Element | null }).webkitFullscreenElement ?? null;

const owns = (scope: RefObject<HTMLElement | null> | undefined) => {
  if (!scope) return true;
  const el = scope.current;
  if (!el?.isConnected || el.getClientRects().length === 0) return false;
  const fs = fullscreenEl();
  return !fs || fs.contains(el);
};

/** ←/→ (and optionally Space) flip between images; ignored while typing or in a dialog. */
export function useFlipKeys(h: FlipKeyHandlers) {
  const latest = useRef(h);
  latest.current = h;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const cur = latest.current;
      if (!cur.enabled || e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey) return;
      if (typing(e.target) || !owns(cur.scope)) return;
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
