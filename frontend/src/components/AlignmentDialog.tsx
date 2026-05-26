import { useEffect, useState } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import Dialog from '@mui/material/Dialog';
import DialogContent from '@mui/material/DialogContent';
import Divider from '@mui/material/Divider';
import Typography from '@mui/material/Typography';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import AutoFixHighRoundedIcon from '@mui/icons-material/AutoFixHighRounded';
import CheckRoundedIcon from '@mui/icons-material/CheckRounded';
import RadioButtonUncheckedRoundedIcon from '@mui/icons-material/RadioButtonUncheckedRounded';
import { AlignableImage, PAIR_COLORS } from './AlignableImage';
import { useAutoWarp } from '../hooks/useAutoWarp';
import { useWarp } from '../hooks/useWarp';
import type { PointPair, Dims } from '../types/api';

interface AlignmentDialogProps {
  open: boolean;
  beforeUrl: string;
  afterUrl: string;
  beforeDims: Dims;
  afterDims: Dims;
  onAligned: (warpedUrl: string) => void;
  onClose: () => void;
}

export function AlignmentDialog({ open, beforeUrl, afterUrl, beforeDims, afterDims, onAligned, onClose }: AlignmentDialogProps) {
  const MAX_PAIRS = 8;
  const MIN_PAIRS = 4;

  const [pairs, setPairs] = useState<PointPair[]>(
    [1, 2, 3, 4].map(id => ({ id, src: null, dst: null }))
  );
  const [autoSummary, setAutoSummary] = useState<string | null>(null);
  const { warp, loading, error } = useWarp();
  const { autoWarp, loading: autoLoading, error: autoError } = useAutoWarp();

  useEffect(() => {
    if (open) {
      setPairs([1, 2, 3, 4].map(id => ({ id, src: null, dst: null })));
      setAutoSummary(null);
    }
  }, [open]);

  const addPair = () => {
    setPairs(prev => {
      if (prev.length >= MAX_PAIRS) return prev;
      return [...prev, { id: prev.length + 1, src: null, dst: null }];
    });
  };

  const completePairs = pairs.filter(p => p.src !== null && p.dst !== null).length;
  const allComplete = completePairs === pairs.length;

  const pendingIdx = pairs.findIndex(p => !p.src || !p.dst);
  const pendingSide: 'src' | 'dst' | null =
    pendingIdx === -1 ? null : (!pairs[pendingIdx].src ? 'src' : 'dst');

  const handlePoint = (side: 'src' | 'dst') => (relX: number, relY: number) => {
    setPairs(prev => {
      const idx = prev.findIndex(p => (side === 'src' ? !p.src : !p.dst));
      if (idx === -1) return prev;
      const next = [...prev];
      next[idx] = { ...next[idx], [side]: { x: relX, y: relY } };
      return next;
    });
  };

  const handleApply = async () => {
    const url = await warp(pairs, beforeDims, afterDims);
    if (url) {
      onAligned(url);
      onClose();
    }
  };

  const handleAutoAlign = async () => {
    if (pairs.some(pair => pair.src || pair.dst) && !window.confirm('Replace the current alignment points with auto-detected points?')) {
      return;
    }

    const result = await autoWarp(beforeDims, afterDims);
    if (!result) return;

    setPairs(result.pairs);
    setAutoSummary(`Auto-detected ${result.pairs.length} pairs from ${result.inlierCount}/${result.matchCount} matches. Confidence ${Math.round(result.confidence * 100)}%.`);
  };

  const handleClose = () => {
    onClose();
  };

  const srcPoints = pairs.map(p => ({ id: p.id, coords: p.src }));
  const dstPoints = pairs.map(p => ({ id: p.id, coords: p.dst }));

  // The active image (needing a click) is always shown large.
  // Images swap: Before is large when placing src, After is large when placing dst.
  const afterIsMain = pendingSide === 'dst';
  const mainUrl    = afterIsMain ? afterUrl    : beforeUrl;
  const mainLabel  = afterIsMain ? 'After'     : 'Before';
  const mainSide   = afterIsMain ? 'dst'       : 'src';
  const mainPoints = afterIsMain ? dstPoints   : srcPoints;
  const refUrl     = afterIsMain ? beforeUrl   : afterUrl;
  const refLabel   = afterIsMain ? 'Before'    : 'After';
  const refSide    = afterIsMain ? 'src'       : 'dst';
  const refPoints  = afterIsMain ? srcPoints   : dstPoints;

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth={false}
      sx={{ '& .MuiDialog-paper': { maxHeight: '97vh', maxWidth: '96vw', m: 1.5 } }}
    >
      <DialogContent sx={{ p: 0, display: 'flex', overflow: 'hidden' }}>

        {/* ── Main image area ── */}
        <Box sx={{ p: 2, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
          <Box>
            <AlignableImage
              imageUrl={mainUrl}
              side={mainSide}
              label={mainLabel}
              points={mainPoints}
              isActive={pendingSide === mainSide}
              onPoint={handlePoint(mainSide)}
            />
          </Box>
        </Box>

        {/* ── Sidebar ── */}
        <Box sx={{
          width: 260,
          flexShrink: 0,
          borderLeft: 1,
          borderColor: 'divider',
          p: 2,
          display: 'flex',
          flexDirection: 'column',
          gap: 1.5,
          overflow: 'auto',
        }}>
          {/* Title + progress + instruction */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
            <Typography variant="subtitle1" fontWeight={700}>
              Align Images
            </Typography>
          </Box>

          <Divider />

          {/* Pair status list */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {pairs.map((p, i) => {
              const color = PAIR_COLORS[i % PAIR_COLORS.length];
              const isCurrent = i === pendingIdx;
              return (
                <Box
                  key={p.id}
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1,
                    px: 1,
                    py: 0.5,
                    borderRadius: 1,
                    bgcolor: isCurrent ? 'action.selected' : 'transparent',
                  }}
                >
                  <Box sx={{ width: 12, height: 12, borderRadius: '50%', bgcolor: color, flexShrink: 0 }} />
                  <Typography variant="body2" sx={{ flex: 1, fontWeight: isCurrent ? 600 : 400 }}>
                    Pair {p.id}
                  </Typography>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
                    {p.src
                      ? <CheckRoundedIcon sx={{ fontSize: 14, color: 'success.main' }} />
                      : <RadioButtonUncheckedRoundedIcon sx={{ fontSize: 14, color: 'text.disabled' }} />
                    }
                    <Typography variant="caption" color="text.secondary" sx={{ fontSize: 10 }}>B</Typography>
                    {p.dst
                      ? <CheckRoundedIcon sx={{ fontSize: 14, color: 'success.main' }} />
                      : <RadioButtonUncheckedRoundedIcon sx={{ fontSize: 14, color: 'text.disabled' }} />
                    }
                    <Typography variant="caption" color="text.secondary" sx={{ fontSize: 10 }}>A</Typography>
                  </Box>
                </Box>
              );
            })}
          </Box>

          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, mt: 0.5 }}>
            {pairs.length < MAX_PAIRS && allComplete && (
              <Button fullWidth startIcon={<AddRoundedIcon />} onClick={addPair}>
                Add Pair
              </Button>
            )}
            <Button
              fullWidth
              disabled={pairs.every(p => !p.src && !p.dst)}
              onClick={() => {
                setPairs([1, 2, 3, 4].map(id => ({ id, src: null, dst: null })));
                setAutoSummary(null);
              }}
            >
              Clear All
            </Button>
            <Button
              fullWidth
              variant="outlined"
              onClick={handleAutoAlign}
              disabled={loading || autoLoading}
              startIcon={autoLoading ? <CircularProgress size={16} color="inherit" /> : <AutoFixHighRoundedIcon />}
            >
              {autoLoading ? 'Finding points…' : 'Auto Align'}
            </Button>
          </Box>

          <Divider />

          {/* Reference thumbnail */}
          <Box>
            <Typography variant="caption" color="text.secondary" fontWeight={600} display="block" gutterBottom>
              Reference
            </Typography>
            <AlignableImage
              imageUrl={refUrl}
              side={refSide}
              label={refLabel}
              points={refPoints}
              isActive={false}
              onPoint={() => {}}
              compact
            />
          </Box>

          {/* Spacer */}
          <Box sx={{ flex: 1 }} />

          {/* Error */}
          {error && <Alert severity="error" sx={{ py: 0.5 }}>{error}</Alert>}
          {autoError && <Alert severity="error" sx={{ py: 0.5 }}>{autoError}</Alert>}
          {autoSummary && <Alert severity="success" sx={{ py: 0.5 }}>{autoSummary}</Alert>}

          <Divider />

          {/* Action buttons */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            <Button fullWidth onClick={handleClose}>Cancel</Button>
            <Button
              fullWidth
              variant="contained"
              onClick={handleApply}
              disabled={completePairs < MIN_PAIRS || loading}
              startIcon={loading ? <CircularProgress size={16} color="inherit" /> : undefined}
            >
              {loading ? 'Processing…' : 'Apply Alignment'}
            </Button>
          </Box>
        </Box>
      </DialogContent>
    </Dialog>
  );
}
