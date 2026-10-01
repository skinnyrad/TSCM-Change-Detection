import { useCallback, useEffect, useRef, useState } from 'react';

type FsElement = HTMLElement & { webkitRequestFullscreen?: () => Promise<void> | void };
type FsDocument = Document & { webkitFullscreenElement?: Element | null; webkitExitFullscreen?: () => void };

const currentFs = () => {
  const d = document as FsDocument;
  return d.fullscreenElement ?? d.webkitFullscreenElement ?? null;
};

/**
 * Fullscreen for a wrapper element (not just an <img>), so overlays, labels and
 * keyboard handlers keep working while fullscreen. Includes the WebKit prefix
 * for Safari.
 */
export function useFullscreen<T extends HTMLElement = HTMLDivElement>() {
  const ref = useRef<T>(null);
  const [active, setActive] = useState(false);

  useEffect(() => {
    const sync = () => setActive(ref.current !== null && currentFs() === ref.current);
    document.addEventListener('fullscreenchange', sync);
    document.addEventListener('webkitfullscreenchange', sync);
    return () => {
      document.removeEventListener('fullscreenchange', sync);
      document.removeEventListener('webkitfullscreenchange', sync);
    };
  }, []);

  const enter = useCallback(() => {
    const el = ref.current as FsElement | null;
    if (!el) return;
    const p = el.requestFullscreen?.() ?? el.webkitRequestFullscreen?.();
    if (p instanceof Promise) p.catch(() => {});
  }, []);

  const exit = useCallback(() => {
    const d = document as FsDocument;
    const p = d.exitFullscreen?.() ?? d.webkitExitFullscreen?.();
    if (p instanceof Promise) p.catch(() => {});
  }, []);

  const toggle = useCallback(() => (currentFs() ? exit() : enter()), [enter, exit]);

  return { ref, active, enter, exit, toggle };
}
