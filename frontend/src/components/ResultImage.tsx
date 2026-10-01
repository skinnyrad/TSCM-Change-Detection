import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import Box from '@mui/material/Box';
import IconButton from '@mui/material/IconButton';
import Paper from '@mui/material/Paper';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import FullscreenRoundedIcon from '@mui/icons-material/FullscreenRounded';
import FullscreenExitRoundedIcon from '@mui/icons-material/FullscreenExitRounded';
import KeyboardArrowLeftRoundedIcon from '@mui/icons-material/KeyboardArrowLeftRounded';
import KeyboardArrowRightRoundedIcon from '@mui/icons-material/KeyboardArrowRightRounded';
import { useAspectRatio, fitWidth } from '../hooks/useAspectRatio';
import { useFlipKeys } from '../hooks/useFlipKeys';
import { useFullscreen } from '../hooks/useFullscreen';
import { ZoomPan } from './ZoomPan';

export interface Frame {
  label: string;
  src: string;
}

interface ResultImageProps {
  src: string;
  caption: string;
  /** Extra images (e.g. Before, After) that ←/→ flips through in fullscreen, before this one. */
  compare?: Frame[];
  /** Overlay (region boxes, ignore zones) drawn over this image, positioned in image fractions. */
  overlay?: ReactNode;
  /** Changing this resets the zoom (e.g. new uploads). The zoom otherwise survives re-analysis and flipping. */
  zoomKey?: unknown;
  /** The overlay handles drags itself (drawing ignore zones), so dragging must not pan. */
  panDisabled?: boolean;
}

const MAX_HEIGHT = '90vh';

export function ResultImage({ src, caption, compare = [], overlay, zoomKey, panDisabled }: ResultImageProps) {
  const { ref, active, toggle } = useFullscreen();
  const { aspectRatio, onLoad } = useAspectRatio();

  const frames = useMemo<Frame[]>(() => [...compare, { label: caption, src }], [compare, caption, src]);
  const last = frames.length - 1;
  const [index, setIndex] = useState(last);
  // Always enter fullscreen showing this result; keep index valid if frames change.
  useEffect(() => { setIndex(last); }, [active, last, src]);

  const step = (d: number) => setIndex(i => (i + d + frames.length) % frames.length);
  useFlipKeys({
    enabled: active && frames.length > 1,
    onPrev: () => step(-1),
    onNext: () => step(1),
    onToggle: () => step(1),
    spaceToggles: true,
    scope: ref,
  });

  const current = frames[Math.min(index, last)] ?? { label: caption, src };
  const showingResult = current.src === src;

  return (
    <Paper
      variant="outlined"
      sx={{
        overflow: 'hidden', display: 'inline-flex', flexDirection: 'column', alignItems: 'center',
        width: 'fit-content', maxWidth: 'calc(100% - 24px)', minWidth: 0,
      }}
    >
      <Box
        ref={ref}
        sx={{
          position: 'relative', width: 'fit-content', maxWidth: '100%',
          '&:hover .fs-btn, &:focus-within .fs-btn': { opacity: 1 },
          '@media (hover: none)': { '& .fs-btn': { opacity: 1 } },
          ...(active && {
            width: '100vw', maxWidth: '100vw', height: '100vh', bgcolor: '#000',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }),
        }}
      >
        <ZoomPan resetKey={zoomKey} panDisabled={panDisabled} sx={{ width: 'fit-content', maxWidth: '100%' }}>
          <Box sx={{ position: 'relative', width: 'fit-content', maxWidth: '100%' }}>
            <Box
              component="img"
              src={current.src}
              alt={current.label}
              onLoad={onLoad}
              draggable={false}
              sx={{
                display: 'block', height: 'auto',
                width: active ? 'auto' : fitWidth(aspectRatio, MAX_HEIGHT),
                maxWidth: active ? '100vw' : '100%',
                maxHeight: active ? '100vh' : MAX_HEIGHT,
              }}
            />
            {showingResult && overlay}
          </Box>
        </ZoomPan>

        {active && (
          <>
            <Typography
              aria-live="polite"
              sx={{ position: 'absolute', top: 12, left: 16, px: 1.5, py: 0.5, borderRadius: 1, bgcolor: 'rgba(0,0,0,0.6)', color: '#fff', fontWeight: 700, fontSize: 14 }}
            >
              {current.label}{frames.length > 1 ? ` (${Math.min(index, last) + 1}/${frames.length})` : ''}
            </Typography>
            {frames.length > 1 && (
              <>
                <IconButton aria-label="Previous image" onClick={() => step(-1)} sx={navSx('left')}>
                  <KeyboardArrowLeftRoundedIcon fontSize="large" />
                </IconButton>
                <IconButton aria-label="Next image" onClick={() => step(1)} sx={navSx('right')}>
                  <KeyboardArrowRightRoundedIcon fontSize="large" />
                </IconButton>
                <Typography sx={{ position: 'absolute', bottom: 12, left: 0, right: 0, textAlign: 'center', color: 'rgba(255,255,255,0.7)', fontSize: 12 }}>
                  ← → flip images · Space next · Esc exit
                </Typography>
              </>
            )}
          </>
        )}

        <Tooltip title={active ? 'Exit fullscreen' : 'Fullscreen'} placement="left">
          <IconButton
            className="fs-btn"
            size="small"
            aria-label={active ? 'Exit fullscreen' : `View ${caption} fullscreen`}
            onClick={toggle}
            sx={{
              position: 'absolute', top: 8, right: 8, opacity: 0, transition: 'opacity 0.15s',
              bgcolor: 'rgba(0,0,0,0.55)', '&:hover': { bgcolor: 'rgba(0,0,0,0.8)' }, '&:focus-visible': { opacity: 1 },
            }}
          >
            {active ? <FullscreenExitRoundedIcon fontSize="small" /> : <FullscreenRoundedIcon fontSize="small" />}
          </IconButton>
        </Tooltip>
      </Box>
      <Typography
        variant="caption" display="block" textAlign="center"
        sx={{ py: 0.75, px: 1, color: 'text.secondary', width: '100%', overflowWrap: 'anywhere' }}
      >
        {caption}
      </Typography>
    </Paper>
  );
}

const navSx = (side: 'left' | 'right') => ({
  position: 'absolute' as const, top: '50%', [side]: 12, transform: 'translateY(-50%)',
  bgcolor: 'rgba(0,0,0,0.5)', color: '#fff', '&:hover': { bgcolor: 'rgba(0,0,0,0.75)' },
});
