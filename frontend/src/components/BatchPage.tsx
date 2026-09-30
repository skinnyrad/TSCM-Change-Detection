import { useMemo, useRef, useState } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import FormControlLabel from '@mui/material/FormControlLabel';
import IconButton from '@mui/material/IconButton';
import LinearProgress from '@mui/material/LinearProgress';
import Paper from '@mui/material/Paper';
import Slider from '@mui/material/Slider';
import Switch from '@mui/material/Switch';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import DownloadRoundedIcon from '@mui/icons-material/DownloadRounded';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import KeyboardArrowLeftRoundedIcon from '@mui/icons-material/KeyboardArrowLeftRounded';
import KeyboardArrowRightRoundedIcon from '@mui/icons-material/KeyboardArrowRightRounded';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import UploadFileRoundedIcon from '@mui/icons-material/UploadFileRounded';
import { useBatch } from '../hooks/useBatch';
import { useFlipKeys } from '../hooks/useFlipKeys';
import {
  REFERENCE_URL, alignedUrl, batchThreshold, buildBatchReportHtml, heatUrl, isAnomalous, toOverlayRegions,
} from '../lib/batch';
import { loadImage, toJpegDataUrl } from '../lib/images';
import type { BatchImageResult, BatchResults } from '../types/api';
import { ImageOverlay } from './ImageOverlay';
import { ResultImage } from './ResultImage';

// ─── Upload ──────────────────────────────────────────────────────────────────

function BatchUpload({ onFiles, disabled }: { onFiles: (f: File[]) => void; disabled: boolean }) {
  const files = useRef<HTMLInputElement>(null);
  const folder = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const take = (list: FileList | null) => { if (list?.length) onFiles(Array.from(list)); };
  return (
    <Paper
      variant="outlined"
      onDragOver={e => { e.preventDefault(); setOver(true); }}
      onDragLeave={() => setOver(false)}
      onDrop={e => { e.preventDefault(); setOver(false); take(e.dataTransfer.files); }}
      sx={{ p: 3, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1.5, borderStyle: 'dashed', borderColor: over ? 'primary.main' : undefined }}
    >
      <UploadFileRoundedIcon sx={{ fontSize: 44, color: 'text.disabled' }} />
      <Typography color="text.secondary" textAlign="center">
        Drop many photos of the <strong>same scene</strong> here (tens to hundreds). The tool learns what the scene normally
        looks like from all of them and flags the shots that differ.
      </Typography>
      <Box sx={{ display: 'flex', gap: 1 }}>
        <Button variant="outlined" disabled={disabled} startIcon={<UploadFileRoundedIcon />} onClick={() => files.current?.click()}>Choose images</Button>
        <Button variant="outlined" disabled={disabled} startIcon={<FolderOpenRoundedIcon />} onClick={() => folder.current?.click()}>Choose folder</Button>
      </Box>
      <input ref={files} type="file" accept="image/jpeg,image/png" multiple hidden onChange={e => { take(e.target.files); e.target.value = ''; }} />
      <input
        ref={el => { folder.current = el; el?.setAttribute('webkitdirectory', ''); }}
        type="file" multiple hidden onChange={e => { take(e.target.files); e.target.value = ''; }}
      />
    </Paper>
  );
}

// ─── Score strip: every image as a dot, sorted by score, with the threshold ──

function ScoreStrip({ images, threshold, selected, onSelect }: {
  images: BatchImageResult[]; threshold: number; selected: string | null; onSelect: (id: string) => void;
}) {
  const W = 1000, H = 120, pad = 8;
  const max = Math.max(threshold * 1.5, ...images.map(i => i.score));
  const y = (s: number) => H - pad - (Math.log1p(Math.max(0, s)) / Math.log1p(max)) * (H - 2 * pad);
  const x = (i: number) => pad + (images.length <= 1 ? 0 : (i / (images.length - 1)) * (W - 2 * pad));
  return (
    <Box component="svg" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" role="img"
      aria-label={`Anomaly scores of ${images.length} images, highest first; dashed line is the threshold`}
      sx={{ width: '100%', height: 120, display: 'block' }}>
      <line x1={0} x2={W} y1={y(threshold)} y2={y(threshold)} stroke="#ff9800" strokeDasharray="6 4" vectorEffect="non-scaling-stroke" />
      {images.map((im, i) => (
        <circle
          key={im.id} cx={x(i)} cy={y(im.score)} r={im.id === selected ? 7 : 4.5}
          fill={im.score >= threshold ? '#ff3c3c' : '#5c9ced'} stroke={im.id === selected ? '#fff' : 'none'} strokeWidth={2}
          style={{ cursor: 'pointer' }} onClick={() => onSelect(im.id)}
        >
          <title>{`${im.name}: score ${im.score.toFixed(1)}`}</title>
        </circle>
      ))}
    </Box>
  );
}

// ─── Result card ─────────────────────────────────────────────────────────────

function ResultCard({ im, anomalous, onOpen, selected }: { im: BatchImageResult; anomalous: boolean; onOpen: () => void; selected: boolean }) {
  return (
    <Paper
      variant="outlined" role="button" tabIndex={0} onClick={onOpen}
      onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onOpen(); } }}
      aria-label={`${im.name}, score ${im.score.toFixed(1)}${anomalous ? ', anomalous' : ''}`}
      sx={{
        overflow: 'hidden', cursor: 'pointer', borderColor: selected ? 'primary.main' : anomalous ? 'error.main' : undefined,
        borderWidth: anomalous || selected ? 2 : 1, '&:focus-visible': { outline: '3px solid #5c9ced' },
      }}
    >
      <Box sx={{ position: 'relative' }}>
        <Box component="img" loading="lazy" src={alignedUrl(im.id, 320)} alt="" sx={{ width: '100%', display: 'block', aspectRatio: '4 / 3', objectFit: 'contain', bgcolor: '#000' }} />
        {anomalous && (
          <Box component="img" loading="lazy" src={heatUrl(im.id)} alt="" sx={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'contain', pointerEvents: 'none' }} />
        )}
      </Box>
      <Box sx={{ p: 1, display: 'flex', alignItems: 'center', gap: 0.75 }}>
        <Typography variant="caption" noWrap sx={{ flex: 1, minWidth: 0 }} title={im.name}>{im.name}</Typography>
        {!im.registered && <Tooltip title={im.message || 'Could not be aligned to the scene'}><Chip size="small" color="warning" label="unaligned" /></Tooltip>}
        <Chip size="small" color={anomalous ? 'error' : 'default'} label={im.score.toFixed(1)} />
      </Box>
    </Paper>
  );
}

// ─── Inspector ───────────────────────────────────────────────────────────────

function Inspector({ im, anomalous, onPrev, onNext, onClose }: {
  im: BatchImageResult; anomalous: boolean; onPrev: () => void; onNext: () => void; onClose: () => void;
}) {
  const [showHeat, setShowHeat] = useState(true);
  const [selectedRank, setSelectedRank] = useState<number | null>(null);
  // ←/→ step between images while not fullscreen (fullscreen uses them to flip image/reference).
  useFlipKeys({
    enabled: true,
    onPrev: () => { if (!document.fullscreenElement) onPrev(); },
    onNext: () => { if (!document.fullscreenElement) onNext(); },
  });
  const compare = useMemo(() => [{ label: 'Golden reference (typical scene)', src: REFERENCE_URL }], []);

  return (
    <Paper variant="outlined" sx={{ p: 2, mb: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1.5, flexWrap: 'wrap' }}>
        <IconButton aria-label="Previous image" onClick={onPrev}><KeyboardArrowLeftRoundedIcon /></IconButton>
        <IconButton aria-label="Next image" onClick={onNext}><KeyboardArrowRightRoundedIcon /></IconButton>
        <Typography variant="subtitle1" sx={{ flex: 1, minWidth: 0 }} noWrap title={im.name}>{im.name}</Typography>
        <Chip color={anomalous ? 'error' : 'success'} label={anomalous ? 'Anomalous' : 'Normal'} />
        <Chip label={`score ${im.score.toFixed(1)} · z ${im.z.toFixed(1)}`} />
        {im.golden && <Chip variant="outlined" label="golden set" />}
        {im.anchor && <Chip variant="outlined" label="reference frame" />}
        <FormControlLabel control={<Switch size="small" checked={showHeat} onChange={e => setShowHeat(e.target.checked)} />} label={<Typography variant="body2">Heat</Typography>} />
        <IconButton aria-label="Close inspector" onClick={onClose}><CloseRoundedIcon /></IconButton>
      </Box>
      {!im.registered && <Alert severity="warning" sx={{ mb: 1.5 }}>This shot could not be aligned to the scene ({im.message || 'no reliable match'}). It may be from a different position or of a different scene — which is itself an anomaly.</Alert>}
      <Box sx={{ display: 'flex', justifyContent: 'center' }}>
        <ResultImage
          key={im.id}
          src={alignedUrl(im.id, 1600)}
          caption={`${im.name} — aligned to the reference frame. ← → previous/next image; fullscreen flips to the golden reference.`}
          compare={compare}
          overlay={
            <>
              {showHeat && <Box component="img" src={heatUrl(im.id)} alt="" sx={{ position: 'absolute', inset: 0, width: '100%', height: '100%', pointerEvents: 'none' }} />}
              <ImageOverlay regions={toOverlayRegions(im.regions)} selectedRank={selectedRank} onSelectRank={setSelectedRank} />
            </>
          }
        />
      </Box>
    </Paper>
  );
}

// ─── Page ────────────────────────────────────────────────────────────────────

/** Draws aligned image + heat + region boxes into one JPEG for the report. */
async function composite(im: BatchImageResult): Promise<string> {
  const [base, heat] = await Promise.all([loadImage(alignedUrl(im.id, 1024)), loadImage(heatUrl(im.id))]);
  const c = document.createElement('canvas');
  c.width = base.naturalWidth;
  c.height = base.naturalHeight;
  const g = c.getContext('2d')!;
  g.drawImage(base, 0, 0);
  g.drawImage(heat, 0, 0, c.width, c.height);
  g.strokeStyle = '#39ff14';
  g.lineWidth = Math.max(2, c.width / 400);
  g.font = `bold ${Math.max(12, c.width / 60)}px sans-serif`;
  g.fillStyle = '#39ff14';
  for (const r of im.regions) {
    g.strokeRect(r.x * c.width, r.y * c.height, r.w * c.width, r.h * c.height);
    g.fillText(String(r.rank), r.x * c.width + 4, r.y * c.height - 4);
  }
  return c.toDataURL('image/jpeg', 0.85);
}

export function BatchPage() {
  const { count, job, results, uploading, error, failed, upload, clear, analyze } = useBatch();
  const [sensitivity, setSensitivity] = useState<number | null>(null);
  const [onlyAnomalies, setOnlyAnomalies] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);

  const running = job.state === 'running';
  const k = sensitivity ?? results?.image_z ?? 6;
  const threshold = results ? batchThreshold(results, k) : Infinity;
  const ranked = results?.images ?? [];
  const anomalies = ranked.filter(im => isAnomalous(im, threshold));
  const shown = onlyAnomalies ? anomalies : ranked;
  const idx = shown.findIndex(im => im.id === selected);
  const current = idx >= 0 ? shown[idx] : null;
  const step = (d: number) => {
    if (shown.length === 0) return;
    const next = shown[((idx < 0 ? 0 : idx) + d + shown.length) % shown.length];
    if (next) setSelected(next.id);
  };

  const exportReport = async (r: BatchResults) => {
    setExporting(true);
    try {
      const [ref, entries] = await Promise.all([
        toJpegDataUrl(REFERENCE_URL, 1200),
        Promise.all(anomalies.slice(0, 100).map(async image => ({ image, preview: await composite(image) }))),
      ]);
      const html = buildBatchReportHtml({ generatedAt: new Date(), results: r, sensitivity: k, threshold, referenceDataUrl: ref, anomalies: entries });
      const a = document.createElement('a');
      a.href = URL.createObjectURL(new Blob([html], { type: 'text/html' }));
      a.download = `tscm-batch-report-${new Date().toISOString().replace(/[:.]/g, '-')}.html`;
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 1000);
    } finally {
      setExporting(false);
    }
  };

  return (
    <Box>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      {failed.length > 0 && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          {failed.length} file{failed.length === 1 ? '' : 's'} could not be read: {failed.slice(0, 3).join('; ')}{failed.length > 3 ? '…' : ''}
        </Alert>
      )}

      <BatchUpload onFiles={upload} disabled={running || uploading !== null} />

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, my: 2, flexWrap: 'wrap' }}>
        <Typography variant="body2" color="text.secondary">{count} image{count === 1 ? '' : 's'} in batch</Typography>
        <Button
          variant="contained" startIcon={<PlayArrowRoundedIcon />} onClick={analyze}
          disabled={count < 3 || running || uploading !== null}
        >
          {results ? 'Re-analyze' : 'Find anomalies'}
        </Button>
        <Button color="warning" onClick={() => { setSelected(null); clear(); }} disabled={count === 0 || running}>Clear</Button>
        {count > 0 && count < 3 && <Typography variant="caption" color="text.secondary">Add at least 3 images (ideally 10+).</Typography>}
        {results && (
          <Button sx={{ ml: 'auto' }} variant="outlined" startIcon={<DownloadRoundedIcon />} disabled={exporting} onClick={() => exportReport(results)}>
            {exporting ? 'Building…' : 'Export report'}
          </Button>
        )}
      </Box>

      {uploading && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="caption" color="text.secondary">Uploading {uploading.done}/{uploading.total}…</Typography>
          <LinearProgress variant="determinate" value={(uploading.done / uploading.total) * 100} />
        </Box>
      )}
      {running && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="caption" color="text.secondary">
            Aligning images to the scene {job.done}/{job.total}{job.done === job.total ? ' — computing statistics…' : '…'}
          </Typography>
          <LinearProgress variant={job.total ? 'determinate' : 'indeterminate'} value={job.total ? (job.done / job.total) * 100 : 0} />
        </Box>
      )}

      {results && (
        <>
          <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: 1.5, mb: 2 }}>
            {[
              [ranked.length, 'images analysed'],
              [anomalies.length, 'anomalous at this sensitivity'],
              [results.golden_count, 'in the golden set'],
              [results.unregistered, 'could not be aligned'],
            ].map(([v, label]) => (
              <Paper key={label as string} variant="outlined" sx={{ p: 1.5, textAlign: 'center' }}>
                <Typography variant="h5" fontWeight={700} color={label === 'anomalous at this sensitivity' && v ? 'error' : 'primary'}>{v}</Typography>
                <Typography variant="caption" color="text.secondary">{label}</Typography>
              </Paper>
            ))}
          </Box>

          <Paper variant="outlined" sx={{ p: 2, mb: 2 }}>
            <Box sx={{ display: 'flex', gap: 3, alignItems: 'center', flexWrap: 'wrap' }}>
              <Box sx={{ flex: '1 1 260px' }}>
                <Typography variant="body2" color="text.secondary" gutterBottom>
                  Strictness: {k.toFixed(1)} (lower flags more images) · threshold {threshold.toFixed(1)}
                </Typography>
                <Slider aria-label="Strictness" size="small" min={2} max={20} step={0.5} value={k} onChange={(_, v) => setSensitivity(v as number)} />
              </Box>
              <FormControlLabel control={<Switch checked={onlyAnomalies} onChange={e => setOnlyAnomalies(e.target.checked)} />} label="Anomalies only" />
            </Box>
            <ScoreStrip images={ranked} threshold={threshold} selected={selected} onSelect={setSelected} />
            <Typography variant="caption" color="text.secondary">Every image, most unusual first. Red = above the threshold. Click a dot to inspect.</Typography>
          </Paper>

          {current && (
            <Inspector im={current} anomalous={isAnomalous(current, threshold)} onPrev={() => step(-1)} onNext={() => step(1)} onClose={() => setSelected(null)} />
          )}

          <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 1.5 }}>
            {shown.map(im => (
              <ResultCard key={im.id} im={im} anomalous={isAnomalous(im, threshold)} selected={im.id === selected}
                onOpen={() => { setSelected(im.id); window.scrollTo({ top: 0, behavior: 'smooth' }); }} />
            ))}
          </Box>
          {shown.length === 0 && <Typography color="text.secondary">No anomalies at this strictness.</Typography>}
        </>
      )}
    </Box>
  );
}
