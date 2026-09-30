import { useEffect, useRef, useState } from 'react';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import ButtonGroup from '@mui/material/ButtonGroup';
import IconButton from '@mui/material/IconButton';
import MuiSlider from '@mui/material/Slider';
import ToggleButton from '@mui/material/ToggleButton';
import ToggleButtonGroup from '@mui/material/ToggleButtonGroup';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import FullscreenRoundedIcon from '@mui/icons-material/FullscreenRounded';
import FullscreenExitRoundedIcon from '@mui/icons-material/FullscreenExitRounded';
import KeyboardArrowLeftRoundedIcon from '@mui/icons-material/KeyboardArrowLeftRounded';
import KeyboardArrowRightRoundedIcon from '@mui/icons-material/KeyboardArrowRightRounded';
import { fitWidth, useAspectRatio } from '../hooks/useAspectRatio';
import { useFlipKeys } from '../hooks/useFlipKeys';
import { useFullscreen } from '../hooks/useFullscreen';

type Mode = 'slider' | 'toggle' | 'auto';

const NORMAL_MAX_HEIGHT = 'min(90vh, calc(100vh - 220px))';
const FULLSCREEN_MAX_HEIGHT = '100vh';

// ─── Comparison slider ───────────────────────────────────────────────────────
// sliderX is a percentage (0–100): the divider and grip share it so they cannot
// drift apart. gripY is independent and lets the user pick which part of the
// image to inspect. The grip is a real keyboard slider (←/→ nudge, Shift = 10%).

interface ComparisonSliderProps {
  beforeUrl: string;
  afterUrl: string;
  sliderX: number;
  onSliderX: (v: number) => void;
  maxHeight: string;
  fullscreen: boolean;
}

function ComparisonSlider({ beforeUrl, afterUrl, sliderX, onSliderX, maxHeight, fullscreen }: ComparisonSliderProps) {
  const [gripY, setGripY] = useState(50);
  const { aspectRatio, onLoad } = useAspectRatio();
  const containerRef = useRef<HTMLDivElement>(null);
  const clamp = (v: number) => Math.max(0, Math.min(100, v));

  // Dragging anywhere on the container (except the grip) moves the divider.
  const startHorizontalDrag = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const container = containerRef.current;
    if (!container) return;
    const updateX = (clientX: number) => {
      const r = container.getBoundingClientRect();
      onSliderX(clamp(((clientX - r.left) / r.width) * 100));
    };
    updateX(e.clientX);
    const onMove = (ev: PointerEvent) => updateX(ev.clientX);
    const onUp = () => {
      document.removeEventListener('pointermove', onMove);
      document.removeEventListener('pointerup', onUp);
    };
    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
  };

  // Dragging the grip moves it in both axes.
  const startGripDrag = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.stopPropagation();
    const container = containerRef.current;
    if (!container) return;
    const onMove = (ev: PointerEvent) => {
      const r = container.getBoundingClientRect();
      onSliderX(clamp(((ev.clientX - r.left) / r.width) * 100));
      setGripY(clamp(((ev.clientY - r.top) / r.height) * 100));
    };
    const onUp = () => {
      document.removeEventListener('pointermove', onMove);
      document.removeEventListener('pointerup', onUp);
    };
    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
  };

  const onGripKey = (e: React.KeyboardEvent) => {
    const step = e.shiftKey ? 10 : 2;
    if (e.key === 'ArrowLeft') onSliderX(clamp(sliderX - step));
    else if (e.key === 'ArrowRight') onSliderX(clamp(sliderX + step));
    else if (e.key === 'ArrowUp') setGripY(clamp(gripY - step));
    else if (e.key === 'ArrowDown') setGripY(clamp(gripY + step));
    else if (e.key === 'Home') onSliderX(0);
    else if (e.key === 'End') onSliderX(100);
    else return;
    e.preventDefault();
  };

  return (
    <Box
      ref={containerRef}
      onPointerDown={startHorizontalDrag}
      sx={{
        position: 'relative', overflow: 'hidden', borderRadius: fullscreen ? 0 : 1, width: 'fit-content',
        maxWidth: fullscreen ? '100vw' : 'calc(100% - 24px)', marginInline: 'auto',
        userSelect: 'none', touchAction: 'none', cursor: 'col-resize',
      }}
    >
      <img
        src={beforeUrl}
        alt="Before"
        onLoad={onLoad}
        style={{ width: fitWidth(aspectRatio, maxHeight), maxWidth: '100%', height: 'auto', maxHeight, display: 'block' }}
      />
      <img
        src={afterUrl}
        alt="After"
        style={{
          position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', objectFit: 'fill',
          clipPath: `inset(0 0 0 ${sliderX}%)`, pointerEvents: 'none',
        }}
      />
      <Box sx={{
        position: 'absolute', top: 0, bottom: 0, left: `${sliderX}%`, width: 2, transform: 'translateX(-50%)',
        bgcolor: 'rgba(255,255,255,0.6)', boxShadow: '0 0 8px rgba(0,0,0,0.5)', pointerEvents: 'none',
      }} />
      <Box
        role="slider"
        tabIndex={0}
        aria-label="Before/After divider"
        aria-orientation="horizontal"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(sliderX)}
        aria-valuetext={sliderX <= 2 ? 'After only' : sliderX >= 98 ? 'Before only' : `${Math.round(sliderX)}% Before`}
        onKeyDown={onGripKey}
        onPointerDown={startGripDrag}
        sx={{
          position: 'absolute', left: `${sliderX}%`, top: `${gripY}%`, transform: 'translate(-50%, -50%)',
          width: 46, height: 46, borderRadius: '50%', bgcolor: 'rgba(255,255,255,0.12)', backdropFilter: 'blur(6px)',
          border: '2px solid rgba(255,255,255,0.65)', display: 'flex', alignItems: 'center', justifyContent: 'center',
          cursor: 'move', boxShadow: '0 2px 12px rgba(0,0,0,0.45)', touchAction: 'none', zIndex: 10,
          '&:hover': { bgcolor: 'rgba(255,255,255,0.22)', borderColor: 'white' },
          '&:focus-visible': { outline: '3px solid #5c9ced', outlineOffset: 2 },
        }}
      >
        <KeyboardArrowLeftRoundedIcon sx={{ color: 'rgba(255,255,255,0.9)', fontSize: 22 }} />
        <KeyboardArrowRightRoundedIcon sx={{ color: 'rgba(255,255,255,0.9)', fontSize: 22 }} />
      </Box>
    </Box>
  );
}

// ─── Stacked image display (toggle and auto modes) ───────────────────────────
// Both images stay mounted and only visibility changes, so a flip never waits on decode.

interface StackedImagesProps {
  beforeUrl: string;
  afterUrl: string;
  showAfter: boolean;
  maxHeight: string;
  fullscreen: boolean;
}

function StackedImages({ beforeUrl, afterUrl, showAfter, maxHeight, fullscreen }: StackedImagesProps) {
  const { aspectRatio, onLoad } = useAspectRatio();
  return (
    <Box sx={{ position: 'relative', width: 'fit-content', maxWidth: fullscreen ? '100vw' : 'calc(100% - 24px)', marginInline: 'auto' }}>
      <Box
        component="img"
        src={beforeUrl}
        alt="Before"
        onLoad={onLoad}
        sx={{
          width: fitWidth(aspectRatio, maxHeight), maxWidth: '100%', height: 'auto', maxHeight,
          borderRadius: fullscreen ? 0 : 1, display: 'block', visibility: showAfter ? 'hidden' : 'visible',
        }}
      />
      <Box
        component="img"
        src={afterUrl}
        alt="After"
        sx={{
          position: 'absolute', top: 0, left: 0, width: '100%', height: '100%',
          borderRadius: fullscreen ? 0 : 1, display: 'block', visibility: showAfter ? 'visible' : 'hidden',
        }}
      />
    </Box>
  );
}

// ─── Tab component ────────────────────────────────────────────────────────────

interface ImageComparisonTabProps {
  beforeUrl: string;
  afterUrl: string;
}

export function ImageComparisonTab({ beforeUrl, afterUrl }: ImageComparisonTabProps) {
  const [mode, setMode] = useState<Mode>('slider');
  const [showAfter, setShowAfter] = useState(false);
  const [sliderX, setSliderX] = useState(50);
  // intervalMs: how long each image is shown. Range 100ms–2000ms, default 500ms.
  const [intervalMs, setIntervalMs] = useState(500);
  const { ref, active, toggle } = useFullscreen();

  // Auto-toggle effect — runs only in 'auto' mode.
  useEffect(() => {
    if (mode !== 'auto') return;
    const id = setInterval(() => setShowAfter(v => !v), intervalMs);
    return () => clearInterval(id);
  }, [mode, intervalMs]);

  const handleModeChange = (_: unknown, v: Mode | null) => {
    if (!v) return;
    setShowAfter(false);
    setMode(v);
  };

  // Keyboard flip: ← shows Before, → shows After, in every mode. Flipping by hand
  // while Auto is running stops the flicker so the chosen image stays put.
  const showSide = (after: boolean) => {
    if (mode === 'auto') setMode('toggle');
    if (mode === 'slider') setSliderX(after ? 0 : 100);
    else setShowAfter(after);
  };
  const isAfter = mode === 'slider' ? sliderX < 50 : showAfter;
  useFlipKeys({
    enabled: true,
    onPrev: () => showSide(false),
    onNext: () => showSide(true),
    onToggle: () => showSide(!isAfter),
    spaceToggles: active,
  });

  const maxHeight = active ? FULLSCREEN_MAX_HEIGHT : NORMAL_MAX_HEIGHT;
  const label = mode === 'slider'
    ? (sliderX >= 98 ? 'Before' : sliderX <= 2 ? 'After' : 'Before | After')
    : (showAfter ? 'After' : 'Before');

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2, flexWrap: 'wrap' }}>
        <Typography variant="body2" color="text.secondary">Comparison Mode:</Typography>
        <ToggleButtonGroup value={mode} exclusive onChange={handleModeChange} size="small" aria-label="Comparison mode">
          <ToggleButton value="slider">Slider</ToggleButton>
          <ToggleButton value="toggle">Toggle</ToggleButton>
          <ToggleButton value="auto">Auto</ToggleButton>
        </ToggleButtonGroup>

        {mode === 'toggle' && (
          <ButtonGroup variant="outlined" size="small">
            <Button onClick={() => setShowAfter(false)} variant={!showAfter ? 'contained' : 'outlined'}>Before</Button>
            <Button onClick={() => setShowAfter(true)} variant={showAfter ? 'contained' : 'outlined'}>After</Button>
            <Button onClick={() => setShowAfter(v => !v)} aria-label="Flip between Before and After">↔</Button>
          </ButtonGroup>
        )}

        {mode === 'auto' && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, minWidth: 220 }}>
            <Typography variant="body2" color="text.secondary" noWrap>Speed:</Typography>
            <MuiSlider
              aria-label="Flicker speed"
              value={2100 - intervalMs}
              onChange={(_, v) => setIntervalMs(2100 - (v as number))}
              min={100} max={2000} step={50}
              valueLabelDisplay="auto"
              valueLabelFormat={() => intervalMs < 1000 ? `${intervalMs}ms` : `${(intervalMs / 1000).toFixed(1)}s`}
              sx={{ flex: 1 }}
            />
          </Box>
        )}

        <Typography variant="caption" color="text.disabled" sx={{ ml: 'auto' }}>
          ← Before · After → &nbsp;(Space flips in fullscreen)
        </Typography>
        <Tooltip title="Fullscreen" placement="left">
          <IconButton size="small" aria-label="View comparison fullscreen" onClick={toggle}>
            <FullscreenRoundedIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      <Box
        ref={ref}
        sx={{
          position: 'relative',
          ...(active && {
            width: '100vw', height: '100vh', bgcolor: '#000', display: 'flex',
            alignItems: 'center', justifyContent: 'center',
          }),
        }}
      >
        {mode === 'slider' ? (
          <ComparisonSlider
            beforeUrl={beforeUrl} afterUrl={afterUrl} sliderX={sliderX} onSliderX={setSliderX}
            maxHeight={maxHeight} fullscreen={active}
          />
        ) : (
          <StackedImages beforeUrl={beforeUrl} afterUrl={afterUrl} showAfter={showAfter} maxHeight={maxHeight} fullscreen={active} />
        )}

        {active && (
          <>
            <Typography
              aria-live="polite"
              sx={{ position: 'absolute', top: 12, left: 16, px: 1.5, py: 0.5, borderRadius: 1, bgcolor: 'rgba(0,0,0,0.6)', color: '#fff', fontWeight: 700, fontSize: 14, pointerEvents: 'none' }}
            >
              {label}
            </Typography>
            <IconButton
              size="small"
              aria-label="Exit fullscreen"
              onClick={toggle}
              sx={{ position: 'absolute', top: 8, right: 12, zIndex: 11, bgcolor: 'rgba(0,0,0,0.55)', '&:hover': { bgcolor: 'rgba(0,0,0,0.8)' } }}
            >
              <FullscreenExitRoundedIcon fontSize="small" />
            </IconButton>
            <IconButton aria-label="Show Before" onClick={() => showSide(false)} sx={navSx('left')}>
              <KeyboardArrowLeftRoundedIcon fontSize="large" />
            </IconButton>
            <IconButton aria-label="Show After" onClick={() => showSide(true)} sx={navSx('right')}>
              <KeyboardArrowRightRoundedIcon fontSize="large" />
            </IconButton>
            <Typography sx={{ position: 'absolute', bottom: 12, left: 0, right: 0, textAlign: 'center', color: 'rgba(255,255,255,0.7)', fontSize: 12 }}>
              ← Before · After → · Space flip · Esc exit
            </Typography>
          </>
        )}
      </Box>
      <Typography variant="caption" color="text.secondary" mt={0.5} display="block" textAlign="center" aria-live="polite">
        {label}
      </Typography>
    </Box>
  );
}

const navSx = (side: 'left' | 'right') => ({
  position: 'absolute' as const, top: '50%', [side]: 12, transform: 'translateY(-50%)', zIndex: 11,
  bgcolor: 'rgba(0,0,0,0.5)', color: '#fff', '&:hover': { bgcolor: 'rgba(0,0,0,0.75)' },
});
