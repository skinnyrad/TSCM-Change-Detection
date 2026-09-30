import { useRef, useState } from 'react';
import Box from '@mui/material/Box';
import type { IgnoreRect, Region } from '../types/api';

interface ImageOverlayProps {
  regions?: Region[];
  selectedRank?: number | null;
  onSelectRank?: (rank: number | null) => void;
  ignore?: IgnoreRect[];
  onIgnoreChange?: (rects: IgnoreRect[]) => void;
  /** When true, dragging on the image draws a new ignore rectangle. */
  drawing?: boolean;
  /** Regions scoring below this fraction of the strongest are drawn faintly. */
  minScore?: number;
}

const pct = (v: number) => `${(v * 100).toFixed(3)}%`;
const clamp01 = (v: number) => Math.max(0, Math.min(1, v));

/** Ranked region boxes and user-drawn ignore zones, positioned in image fractions. */
export function ImageOverlay({
  regions = [], selectedRank = null, onSelectRank, ignore = [], onIgnoreChange, drawing = false, minScore = 0.25,
}: ImageOverlayProps) {
  const ref = useRef<HTMLDivElement>(null);
  const [draft, setDraft] = useState<IgnoreRect | null>(null);

  const point = (e: React.PointerEvent) => {
    const r = ref.current!.getBoundingClientRect();
    return [clamp01((e.clientX - r.left) / r.width), clamp01((e.clientY - r.top) / r.height)] as const;
  };

  const onDown = (e: React.PointerEvent) => {
    if (!drawing) return;
    e.preventDefault();
    e.currentTarget.setPointerCapture(e.pointerId);
    const [x, y] = point(e);
    setDraft([x, y, x, y]);
  };
  const onMove = (e: React.PointerEvent) => {
    if (!draft) return;
    const [x, y] = point(e);
    setDraft([draft[0], draft[1], x, y]);
  };
  const onUp = () => {
    if (!draft) return;
    const [ax, ay, bx, by] = draft;
    const rect: IgnoreRect = [Math.min(ax, bx), Math.min(ay, by), Math.max(ax, bx), Math.max(ay, by)];
    setDraft(null);
    if (rect[2] - rect[0] > 0.01 && rect[3] - rect[1] > 0.01) onIgnoreChange?.([...ignore, rect]);
  };

  const box = (r: IgnoreRect) => ({ left: pct(r[0]), top: pct(r[1]), width: pct(r[2] - r[0]), height: pct(r[3] - r[1]) });

  return (
    <Box
      ref={ref}
      onPointerDown={onDown}
      onPointerMove={onMove}
      onPointerUp={onUp}
      sx={{ position: 'absolute', inset: 0, cursor: drawing ? 'crosshair' : 'default', touchAction: drawing ? 'none' : 'auto', pointerEvents: drawing ? 'auto' : 'none' }}
    >
      {regions.map(r => {
        const strong = r.score >= minScore;
        const selected = selectedRank === r.rank;
        return (
          <Box
            key={r.rank}
            role="button"
            tabIndex={drawing ? -1 : 0}
            aria-label={`Region ${r.rank}, ${r.area_pct.toFixed(2)}% of the image`}
            aria-pressed={selected}
            onClick={() => onSelectRank?.(selected ? null : r.rank)}
            onKeyDown={e => { if (e.key === 'Enter') onSelectRank?.(selected ? null : r.rank); }}
            sx={{
              position: 'absolute', ...box([r.x, r.y, r.x + r.w, r.y + r.h]),
              // Sizes divide by --zoom (set by ZoomPan) so lines and labels stay
              // the same size on screen when the image is zoomed.
              border: 'calc(2px / var(--zoom, 1)) solid', borderColor: selected ? '#fff' : strong ? '#39ff14' : 'rgba(57,255,20,0.35)',
              boxShadow: selected ? '0 0 0 2px rgba(0,0,0,0.6)' : 'none',
              pointerEvents: drawing ? 'none' : 'auto', cursor: 'pointer', boxSizing: 'border-box',
              '&:focus-visible': { outline: '2px solid #fff' },
            }}
          >
            <Box component="span" sx={{
              position: 'absolute', top: -1, left: -1, px: 0.5, fontSize: 11, lineHeight: '16px', fontWeight: 700,
              transform: 'scale(calc(1 / var(--zoom, 1)))', transformOrigin: '0 0',
              bgcolor: strong ? '#39ff14' : 'rgba(57,255,20,0.6)', color: '#000', userSelect: 'none',
            }}>{r.rank}</Box>
          </Box>
        );
      })}

      {ignore.map((r, i) => (
        <Box key={i} sx={{
          position: 'absolute', ...box(r), border: 'calc(2px / var(--zoom, 1)) dashed #ff9800', boxSizing: 'border-box',
          background: 'repeating-linear-gradient(45deg, rgba(255,152,0,0.18) 0 6px, transparent 6px 12px)',
          pointerEvents: 'none',
        }}>
          {onIgnoreChange && (
            <Box
              component="button"
              aria-label={`Remove ignore zone ${i + 1}`}
              onPointerDown={e => e.stopPropagation()}
              onClick={() => onIgnoreChange(ignore.filter((_, j) => j !== i))}
              sx={{ position: 'absolute', top: 0, right: 0, pointerEvents: 'auto', cursor: 'pointer', border: 0, bgcolor: '#ff9800', color: '#000', fontSize: 12, lineHeight: '16px', px: 0.5, transform: 'scale(calc(1 / var(--zoom, 1)))', transformOrigin: '100% 0' }}
            >×</Box>
          )}
        </Box>
      ))}
      {draft && (
        <Box sx={{ position: 'absolute', ...box([Math.min(draft[0], draft[2]), Math.min(draft[1], draft[3]), Math.max(draft[0], draft[2]), Math.max(draft[1], draft[3])]), border: 'calc(2px / var(--zoom, 1)) dashed #ff9800', pointerEvents: 'none' }} />
      )}
    </Box>
  );
}
