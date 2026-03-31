import { useEffect, useRef, useState } from 'react';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import ButtonGroup from '@mui/material/ButtonGroup';
import MuiSlider from '@mui/material/Slider';
import ToggleButton from '@mui/material/ToggleButton';
import ToggleButtonGroup from '@mui/material/ToggleButtonGroup';
import Typography from '@mui/material/Typography';
import KeyboardArrowLeftRoundedIcon from '@mui/icons-material/KeyboardArrowLeftRounded';
import KeyboardArrowRightRoundedIcon from '@mui/icons-material/KeyboardArrowRightRounded';

// ─── Custom comparison slider ────────────────────────────────────────────────
// sliderX and gripY are both percentages (0–100).
// sliderX drives both the divider line and the grip's horizontal position,
// so they are structurally identical — they can never drift apart.
// gripY is independent and lets the user pick which part of the image to inspect.

interface ComparisonSliderProps {
  beforeUrl: string;
  afterUrl: string;
}

function ComparisonSlider({ beforeUrl, afterUrl }: ComparisonSliderProps) {
  const [sliderX, setSliderX] = useState(50);
  const [gripY, setGripY] = useState(50);
  const containerRef = useRef<HTMLDivElement>(null);

  // Dragging anywhere on the container (except the grip) moves the divider.
  const startHorizontalDrag = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const container = containerRef.current;
    if (!container) return;

    const updateX = (clientX: number) => {
      const r = container.getBoundingClientRect();
      setSliderX(Math.max(0, Math.min(100, ((clientX - r.left) / r.width) * 100)));
    };

    updateX(e.clientX); // jump to click position immediately

    const onMove = (ev: PointerEvent) => updateX(ev.clientX);
    const onUp = () => {
      document.removeEventListener('pointermove', onMove);
      document.removeEventListener('pointerup', onUp);
    };
    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
  };

  // Dragging the grip moves it in both axes. stopPropagation prevents the
  // container's horizontal-only handler from also firing.
  const startGripDrag = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.stopPropagation();
    const container = containerRef.current;
    if (!container) return;

    const onMove = (ev: PointerEvent) => {
      const r = container.getBoundingClientRect();
      setSliderX(Math.max(0, Math.min(100, ((ev.clientX - r.left) / r.width) * 100)));
      setGripY(Math.max(0, Math.min(100, ((ev.clientY - r.top) / r.height) * 100)));
    };
    const onUp = () => {
      document.removeEventListener('pointermove', onMove);
      document.removeEventListener('pointerup', onUp);
    };
    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
  };

  return (
    <Box
      ref={containerRef}
      onPointerDown={startHorizontalDrag}
      sx={{
        position: 'relative',
        overflow: 'hidden',
        borderRadius: 1,
        width: 'fit-content',
        maxWidth: 'calc(100% - 24px)',
        marginInline: 'auto',
        userSelect: 'none',
        touchAction: 'none',
        cursor: 'col-resize',
      }}
    >
      {/* Before image — in normal flow, establishes container height */}
      <img
        src={beforeUrl}
        alt="Before"
        style={{ width: 'auto', maxWidth: '100%', height: '90vh', display: 'block' }}
      />

      {/* After image — same size, clipped to show only the right portion */}
      <img
        src={afterUrl}
        alt="After"
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          objectFit: 'fill',
          clipPath: `inset(0 0 0 ${sliderX}%)`,
          pointerEvents: 'none',
        }}
      />

      {/* Divider line — shares sliderX, always aligned with grip */}
      <Box sx={{
        position: 'absolute',
        top: 0,
        bottom: 0,
        left: `${sliderX}%`,
        width: 2,
        transform: 'translateX(-50%)',
        bgcolor: 'rgba(255,255,255,0.6)',
        boxShadow: '0 0 8px rgba(0,0,0,0.5)',
        pointerEvents: 'none',
      }} />

      {/* Grip — always at sliderX horizontally; gripY controls vertical position */}
      <Box
        onPointerDown={startGripDrag}
        sx={{
          position: 'absolute',
          left: `${sliderX}%`,
          top: `${gripY}%`,
          transform: 'translate(-50%, -50%)',
          width: 46,
          height: 46,
          borderRadius: '50%',
          bgcolor: 'rgba(255,255,255,0.12)',
          backdropFilter: 'blur(6px)',
          border: '2px solid rgba(255,255,255,0.65)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          cursor: 'move',
          boxShadow: '0 2px 12px rgba(0,0,0,0.45)',
          touchAction: 'none',
          zIndex: 10,
          '&:hover': {
            bgcolor: 'rgba(255,255,255,0.22)',
            borderColor: 'white',
          },
        }}
      >
        <KeyboardArrowLeftRoundedIcon sx={{ color: 'rgba(255,255,255,0.9)', fontSize: 22 }} />
        <KeyboardArrowRightRoundedIcon sx={{ color: 'rgba(255,255,255,0.9)', fontSize: 22 }} />
      </Box>
    </Box>
  );
}

// ─── Stacked image display (shared by toggle and auto modes) ─────────────────

interface StackedImagesProps {
  beforeUrl: string;
  afterUrl: string;
  showAfter: boolean;
}

function StackedImages({ beforeUrl, afterUrl, showAfter }: StackedImagesProps) {
  return (
    <Box sx={{ position: 'relative', width: 'fit-content', maxWidth: 'calc(100% - 24px)', marginInline: 'auto' }}>
      <Box
        component="img"
        src={beforeUrl}
        alt="Before"
        sx={{
          width: 'auto',
          maxWidth: '100%',
          height: '90vh',
          borderRadius: 1,
          display: 'block',
          visibility: showAfter ? 'hidden' : 'visible',
        }}
      />
      <Box
        component="img"
        src={afterUrl}
        alt="After"
        sx={{
          position: 'absolute',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          borderRadius: 1,
          display: 'block',
          visibility: showAfter ? 'visible' : 'hidden',
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
  const [mode, setMode] = useState<'slider' | 'toggle' | 'auto'>('slider');
  const [showAfter, setShowAfter] = useState(false);
  // intervalMs: how long each image is shown. Range 100ms–2000ms, default 500ms.
  const [intervalMs, setIntervalMs] = useState(500);

  // Auto-toggle effect — runs only in 'auto' mode.
  useEffect(() => {
    if (mode !== 'auto') return;
    const id = setInterval(() => setShowAfter(v => !v), intervalMs);
    return () => clearInterval(id);
  }, [mode, intervalMs]);

  // Reset showAfter when leaving auto/toggle so state is predictable.
  const handleModeChange = (_: unknown, v: string | null) => {
    if (!v) return;
    setShowAfter(false);
    setMode(v as 'slider' | 'toggle' | 'auto');
  };

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2, flexWrap: 'wrap' }}>
        <Typography variant="body2" color="text.secondary">
          Comparison Mode:
        </Typography>
        <ToggleButtonGroup
          value={mode}
          exclusive
          onChange={handleModeChange}
          size="small"
        >
          <ToggleButton value="slider">Slider</ToggleButton>
          <ToggleButton value="toggle">Toggle</ToggleButton>
          <ToggleButton value="auto">Auto</ToggleButton>
        </ToggleButtonGroup>

        {mode === 'toggle' && (
          <ButtonGroup variant="outlined" size="small">
            <Button onClick={() => setShowAfter(false)} variant={!showAfter ? 'contained' : 'outlined'}>
              Before
            </Button>
            <Button onClick={() => setShowAfter(true)} variant={showAfter ? 'contained' : 'outlined'}>
              After
            </Button>
            <Button onClick={() => setShowAfter(v => !v)}>↔</Button>
          </ButtonGroup>
        )}

        {mode === 'auto' && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, minWidth: 220 }}>
            <Typography variant="body2" color="text.secondary" noWrap>
              Speed:
            </Typography>
            <MuiSlider
              value={2100 - intervalMs}
              onChange={(_, v) => setIntervalMs(2100 - (v as number))}
              min={100}
              max={2000}
              step={50}
              valueLabelDisplay="auto"
              valueLabelFormat={() => intervalMs < 1000 ? `${intervalMs}ms` : `${(intervalMs / 1000).toFixed(1)}s`}
              sx={{ flex: 1 }}
            />
          </Box>
        )}
      </Box>

      {mode === 'slider' ? (
        <ComparisonSlider beforeUrl={beforeUrl} afterUrl={afterUrl} />
      ) : (
        <Box>
          <StackedImages beforeUrl={beforeUrl} afterUrl={afterUrl} showAfter={showAfter} />
          <Typography variant="caption" color="text.secondary" mt={0.5} display="block" textAlign="center">
            {showAfter ? 'After' : 'Before'}
          </Typography>
        </Box>
      )}
    </Box>
  );
}
