import { useEffect, useState } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import Dialog from '@mui/material/Dialog';
import DialogContent from '@mui/material/DialogContent';
import Divider from '@mui/material/Divider';
import Typography from '@mui/material/Typography';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded';
import CheckCircleRoundedIcon from '@mui/icons-material/CheckCircleRounded';
import CheckRoundedIcon from '@mui/icons-material/CheckRounded';
import RadioButtonUncheckedRoundedIcon from '@mui/icons-material/RadioButtonUncheckedRounded';
import { AlignableImage, PAIR_COLORS } from './AlignableImage';
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
  const [warpedUrl, setWarpedUrl] = useState<string | null>(null);
  const { warp, loading, error } = useWarp();

  useEffect(() => {
    if (open) {
      setPairs([1, 2, 3, 4].map(id => ({ id, src: null, dst: null })));
      setWarpedUrl(prev => { if (prev) URL.revokeObjectURL(prev); return null; });
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
    if (url) setWarpedUrl(url);
  };

  const handleConfirm = () => {
    if (warpedUrl) {
      onAligned(warpedUrl);
      onClose();
    }
  };

  const handleClose = () => {
    if (warpedUrl) URL.revokeObjectURL(warpedUrl);
    setWarpedUrl(null);
    onClose();
  };

  const srcPoints = pairs.map(p => ({ id: p.id, coords: p.src }));
  const dstPoints = pairs.map(p => ({ id: p.id, coords: p.dst }));
  const showPreview = warpedUrl !== null;

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

  const nextPairLabel = pendingIdx !== -1
    ? `Place point ${pairs[pendingIdx].id} on ${pendingSide === 'src' ? 'Before' : 'After'}`
    : 'All pairs placed';

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
          {showPreview ? (
            <Box>
              <Alert severity="info" sx={{ mb: 2 }}>
                Review the alignment. The warped Before image should match the After geometry. Confirm if it looks correct.
              </Alert>
              <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-start' }}>
                <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                  <Typography variant="subtitle1" fontWeight={600} textAlign="center" sx={{ mb: 1 }}>
                    Warped Before
                  </Typography>
                  <Box component="img" src={warpedUrl!} alt="Warped before"
                    sx={{ display: 'block', width: 'auto', height: 'auto', maxWidth: 'calc(48vw - 160px)', maxHeight: 'calc(97vh - 160px)', borderRadius: 1 }} />
                </Box>
                <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                  <Typography variant="subtitle1" fontWeight={600} textAlign="center" sx={{ mb: 1 }}>
                    After
                  </Typography>
                  <Box component="img" src={afterUrl} alt="After"
                    sx={{ display: 'block', width: 'auto', height: 'auto', maxWidth: 'calc(48vw - 160px)', maxHeight: 'calc(97vh - 160px)', borderRadius: 1 }} />
                </Box>
              </Box>
            </Box>
          ) : (
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
          )}
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
              {showPreview ? 'Alignment Preview' : 'Align Images'}
            </Typography>
            <Chip
              label={`${completePairs} / ${pairs.length} pairs complete`}
              color={allComplete ? 'success' : 'default'}
              size="small"
              sx={{ alignSelf: 'flex-start' }}
            />
            {!showPreview && (
              pendingSide !== null ? (
                <Alert severity="info" sx={{ py: 0.5, px: 1.5 }}>
                  Click <strong>{mainLabel}</strong> to place point {pairs[pendingIdx]?.id}
                </Alert>
              ) : (
                <Alert severity="success" sx={{ py: 0.5, px: 1.5 }}>
                  All pairs placed
                </Alert>
              )
            )}
          </Box>

          <Divider />

          {!showPreview && (
            <>
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
            </>
          )}

          {/* Spacer */}
          <Box sx={{ flex: 1 }} />

          {/* Error */}
          {error && <Alert severity="error" sx={{ py: 0.5 }}>{error}</Alert>}

          <Divider />

          {/* Action buttons */}
          {showPreview ? (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              <Button
                fullWidth
                startIcon={<ArrowBackRoundedIcon />}
                onClick={() => setWarpedUrl(prev => { if (prev) URL.revokeObjectURL(prev); return null; })}
              >
                Adjust Points
              </Button>
              <Button fullWidth onClick={handleClose}>Cancel</Button>
              <Button
                fullWidth
                variant="contained"
                color="success"
                startIcon={<CheckCircleRoundedIcon />}
                onClick={handleConfirm}
              >
                Confirm Alignment
              </Button>
            </Box>
          ) : (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              <Button
                fullWidth
                disabled={pairs.every(p => !p.src && !p.dst)}
                onClick={() => setPairs([1, 2, 3, 4].map(id => ({ id, src: null, dst: null })))}
              >
                Clear All
              </Button>
              {pairs.length < MAX_PAIRS && allComplete && (
                <Button fullWidth startIcon={<AddRoundedIcon />} onClick={addPair}>
                  Add Pair
                </Button>
              )}
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
          )}
        </Box>
      </DialogContent>
    </Dialog>
  );
}
