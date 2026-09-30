import { useCallback, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import Box from '@mui/material/Box';
import IconButton from '@mui/material/IconButton';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import type { SxProps, Theme } from '@mui/material/styles';
import ZoomInRoundedIcon from '@mui/icons-material/ZoomInRounded';
import ZoomOutRoundedIcon from '@mui/icons-material/ZoomOutRounded';
import FitScreenRoundedIcon from '@mui/icons-material/FitScreenRounded';
import { IDENTITY, panBy, visibleRect, zoomAt, type View } from '../lib/zoom';

interface ZoomPanProps {
  children: ReactNode;
  /** Changing this resets the view (e.g. when new images are uploaded). */
  resetKey?: unknown;
  /** Dragging is handled by the content (e.g. drawing ignore zones), so don't pan. */
  panDisabled?: boolean;
  sx?: SxProps<Theme>;
}

const STEP = 1.5;
const HINT = 'Pinch or ⌘/Ctrl + scroll to zoom · drag or scroll to pan · double-click to zoom in/out · + − 0 keys';

// Elements that own their pointer gestures: never start a pan on them.
const interactive = (t: EventTarget | null) =>
  !!(t as HTMLElement | null)?.closest?.('button, a, input, [role="slider"], [data-no-pan]');

const typing = (t: EventTarget | null) => {
  const el = t as HTMLElement | null;
  return !!el?.tagName && (el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName));
};

/**
 * Zoom and pan for images. The content is laid out normally (so it keeps its
 * natural size) and a transform is applied on top; the view therefore survives
 * swapping the content — flipping Before/After, changing comparison modes or
 * re-running detection keeps the same zoomed area on screen.
 *
 * The current scale is exposed to the content as the CSS variable `--zoom`, so
 * overlays can keep lines and labels a constant on-screen size.
 */
export function ZoomPan({ children, resetKey, panDisabled = false, sx }: ZoomPanProps) {
  const outer = useRef<HTMLDivElement>(null);
  const [view, setView] = useState<View>(IDENTITY);
  const [hover, setHover] = useState(false);
  const viewRef = useRef(view);
  viewRef.current = view;
  const zoomed = view.s > 1.001;

  useEffect(() => { setView(IDENTITY); }, [resetKey]);

  const size = () => [outer.current?.clientWidth ?? 1, outer.current?.clientHeight ?? 1] as const;
  const local = (clientX: number, clientY: number) => {
    const r = outer.current!.getBoundingClientRect();
    return [clientX - r.left, clientY - r.top] as const;
  };
  const zoomBy = useCallback((factor: number, px?: number, py?: number) => {
    const [w, h] = size();
    setView(v => zoomAt(v, factor, px ?? w / 2, py ?? h / 2, w, h));
  }, []);
  const reset = useCallback(() => setView(IDENTITY), []);

  // Wheel: pinch (reported as ctrl+wheel) or ⌘/Ctrl+scroll zooms at the cursor;
  // plain scrolling pans while zoomed and scrolls the page otherwise.
  useEffect(() => {
    const el = outer.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      const [w, h] = size();
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault();
        const [px, py] = local(e.clientX, e.clientY);
        // Trackpad pinches send small deltas; a mouse-wheel notch sends ~100.
        // Cap each event so one notch is a ~25% step, not a 4× jump.
        const d = Math.max(-25, Math.min(25, e.deltaMode === 1 ? e.deltaY * 16 : e.deltaY));
        // Functional updates so several events in one frame accumulate.
        setView(v => zoomAt(v, Math.exp(-d * 0.01), px, py, w, h));
      } else if (viewRef.current.s > 1.001) {
        e.preventDefault();
        setView(v => panBy(v, -e.deltaX, -e.deltaY, w, h));
      }
    };
    el.addEventListener('wheel', onWheel, { passive: false });
    return () => el.removeEventListener('wheel', onWheel);
  }, []);

  // Drag to pan (mouse/pen/one finger) and two-finger pinch. A pan only starts
  // after a few pixels of movement, so clicks on content (e.g. region boxes)
  // still work; the click that ends a pan is swallowed.
  const pointers = useRef(new Map<number, { x: number; y: number }>());
  const gesture = useRef<{ lastX: number; lastY: number; moved: number; pinch?: number } | null>(null);
  const swallowClick = useRef(false);

  const onPointerDown = (e: React.PointerEvent) => {
    if (e.defaultPrevented || interactive(e.target) || (e.pointerType === 'mouse' && e.button !== 0)) return;
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (pointers.current.size === 2) {
      const [a, b] = [...pointers.current.values()] as [{ x: number; y: number }, { x: number; y: number }];
      gesture.current = { lastX: (a.x + b.x) / 2, lastY: (a.y + b.y) / 2, moved: 99, pinch: Math.hypot(a.x - b.x, a.y - b.y) };
    } else if (!panDisabled) {
      gesture.current = { lastX: e.clientX, lastY: e.clientY, moved: 0 };
    }
  };

  useEffect(() => {
    const onMove = (e: PointerEvent) => {
      if (!pointers.current.has(e.pointerId)) return;
      pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
      const g = gesture.current;
      if (!g) return;
      const [w, h] = size();
      if (g.pinch !== undefined && pointers.current.size >= 2) {
        const [a, b] = [...pointers.current.values()] as [{ x: number; y: number }, { x: number; y: number }];
        const mx = (a.x + b.x) / 2, my = (a.y + b.y) / 2, d = Math.hypot(a.x - b.x, a.y - b.y);
        const [px, py] = local(mx, my);
        setView(v => panBy(zoomAt(v, d / (g.pinch || d), px, py, w, h), mx - g.lastX, my - g.lastY, w, h));
        Object.assign(g, { lastX: mx, lastY: my, pinch: d });
        return;
      }
      const dx = e.clientX - g.lastX, dy = e.clientY - g.lastY;
      g.moved += Math.abs(dx) + Math.abs(dy);
      g.lastX = e.clientX;
      g.lastY = e.clientY;
      if (g.moved > 4 && viewRef.current.s > 1.001) {
        swallowClick.current = true;
        setView(v => panBy(v, dx, dy, w, h));
      }
    };
    const onUp = (e: PointerEvent) => {
      pointers.current.delete(e.pointerId);
      if (pointers.current.size === 0) gesture.current = null;
      else if (gesture.current?.pinch !== undefined) gesture.current = null; // lifting one finger ends the pinch
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
    window.addEventListener('pointercancel', onUp);
    return () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      window.removeEventListener('pointercancel', onUp);
    };
  }, []);

  // Keyboard: + / − / 0 while the pointer is over the image, focus is inside
  // it, or it is fullscreen (so several zoomable images on a page don't all react).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = outer.current;
      if (!el || e.altKey || e.ctrlKey || e.metaKey || typing(e.target)) return;
      const active = hover || el.contains(document.activeElement) || !!document.fullscreenElement?.contains(el);
      if (!active) return;
      if (e.key === '+' || e.key === '=') zoomBy(STEP);
      else if (e.key === '-' || e.key === '_') zoomBy(1 / STEP);
      else if (e.key === '0') reset();
      else return;
      e.preventDefault();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [hover, zoomBy, reset]);

  const vis = visibleRect(view, outer.current?.clientWidth ?? 1, outer.current?.clientHeight ?? 1);

  return (
    <Box
      ref={outer}
      className="zoom-pan"
      onPointerDown={onPointerDown}
      onPointerEnter={() => setHover(true)}
      onPointerLeave={() => setHover(false)}
      onScroll={e => {
        // Fallback for browsers without overflow: clip.
        e.currentTarget.scrollTop = 0;
        e.currentTarget.scrollLeft = 0;
      }}
      onClickCapture={e => {
        if (swallowClick.current) {
          swallowClick.current = false;
          e.stopPropagation();
          e.preventDefault();
        }
      }}
      onDoubleClick={e => {
        if (interactive(e.target) || panDisabled) return;
        const [px, py] = local(e.clientX, e.clientY);
        if (zoomed) reset();
        else zoomBy(2.5, px, py);
      }}
      sx={{
        position: 'relative',
        // "clip", not "hidden": a hidden-overflow box can still be scrolled by
        // the browser (e.g. when focus moves to the slider grip inside it),
        // which would shift the content and the zoom controls out of place.
        overflow: 'clip',
        // Let the page scroll vertically on touch until zoomed in; then all touch gestures pan/zoom.
        touchAction: zoomed ? 'none' : 'pan-y',
        cursor: zoomed && !panDisabled ? 'grab' : undefined,
        '&:active': zoomed && !panDisabled ? { cursor: 'grabbing' } : undefined,
        '&:hover .zoom-ui, &:focus-within .zoom-ui': { opacity: 1 },
        '@media (hover: none)': { '& .zoom-ui': { opacity: 1 } },
        ...sx,
      }}
    >
      <Box
        style={{ transform: `translate(${view.x}px, ${view.y}px) scale(${view.s})`, ['--zoom' as string]: view.s }}
        sx={{ transformOrigin: '0 0' }}
      >
        {children}
      </Box>

      {/* Locator: where the zoomed view sits in the whole image. */}
      {zoomed && (
        <Box aria-hidden sx={{
          position: 'absolute', left: 8, bottom: 8, width: 72, height: 54, border: '1px solid rgba(255,255,255,0.7)',
          bgcolor: 'rgba(0,0,0,0.45)', borderRadius: 0.5, pointerEvents: 'none', zIndex: 12,
        }}>
          <Box sx={{
            position: 'absolute', border: '2px solid #5c9ced', bgcolor: 'rgba(92,156,237,0.25)',
            left: `${vis.x * 100}%`, top: `${vis.y * 100}%`, width: `${vis.w * 100}%`, height: `${vis.h * 100}%`,
          }} />
        </Box>
      )}

      <Box
        className="zoom-ui"
        data-no-pan
        sx={{
          position: 'absolute', right: 8, bottom: 8, zIndex: 12, display: 'flex', alignItems: 'center',
          bgcolor: 'rgba(0,0,0,0.6)', borderRadius: 5, px: 0.5, opacity: zoomed ? 1 : 0, transition: 'opacity 0.15s',
        }}
      >
        <IconButton size="small" aria-label="Zoom out" onClick={() => zoomBy(1 / STEP)} disabled={!zoomed} sx={{ color: '#fff' }}>
          <ZoomOutRoundedIcon fontSize="small" />
        </IconButton>
        <Tooltip title={HINT} placement="top">
          <Typography component="span" aria-live="polite" sx={{ color: '#fff', fontSize: 12, minWidth: 40, textAlign: 'center', userSelect: 'none' }}>
            {Math.round(view.s * 100)}%
          </Typography>
        </Tooltip>
        <IconButton size="small" aria-label="Zoom in" onClick={() => zoomBy(STEP)} sx={{ color: '#fff' }}>
          <ZoomInRoundedIcon fontSize="small" />
        </IconButton>
        <IconButton size="small" aria-label="Reset zoom" onClick={reset} disabled={!zoomed} sx={{ color: '#fff' }}>
          <FitScreenRoundedIcon fontSize="small" />
        </IconButton>
      </Box>
    </Box>
  );
}
