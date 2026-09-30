import { useEffect, useMemo, useState } from 'react';
import Accordion from '@mui/material/Accordion';
import AccordionDetails from '@mui/material/AccordionDetails';
import AccordionSummary from '@mui/material/AccordionSummary';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Divider from '@mui/material/Divider';
import FormControlLabel from '@mui/material/FormControlLabel';
import LinearProgress from '@mui/material/LinearProgress';
import Skeleton from '@mui/material/Skeleton';
import Slider from '@mui/material/Slider';
import Switch from '@mui/material/Switch';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import DownloadRoundedIcon from '@mui/icons-material/DownloadRounded';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import { useAnalyze } from '../hooks/useAnalyze';
import { postForm } from '../lib/api';
import { cropDataUrl, sha256Hex, toJpegDataUrl } from '../lib/images';
import { buildReportHtml } from '../lib/report';
import { HIGHLIGHT_PRESETS, type DetectionSettings } from '../lib/settings';
import { ImageOverlay } from './ImageOverlay';
import { RegionsPanel } from './RegionsPanel';
import { ResultImage } from './ResultImage';
import { StatsBar } from './StatsBar';

interface ChangeDetectionTabProps {
  ready: boolean;
  imageKey: string;
  settings: DetectionSettings;
  onSettings: (patch: Partial<DetectionSettings>) => void;
  beforeUrl: string;
  afterUrl: string;
  beforeFile: File | null;
  afterFile: File | null;
  /** Called after a server-side alignment setting changed, so analysis re-runs. */
  onRefresh: () => void;
}

function Labeled({ label, children, min = 160 }: { label: string; children: React.ReactNode; min?: number }) {
  return (
    <Box sx={{ flex: 1, minWidth: min }}>
      <Typography variant="body2" color="text.secondary" gutterBottom>{label}</Typography>
      {children}
    </Box>
  );
}

function Toggle({ label, checked, onChange, hint }: { label: string; checked: boolean; onChange: (v: boolean) => void; hint?: string }) {
  const control = (
    <FormControlLabel
      control={<Switch checked={checked} onChange={e => onChange(e.target.checked)} size="small" />}
      label={<Typography variant="body2" color="text.secondary">{label}</Typography>}
    />
  );
  return hint ? <Tooltip title={hint} placement="top">{control}</Tooltip> : control;
}

export function ChangeDetectionTab({
  ready, imageKey, settings, onSettings, beforeUrl, afterUrl, beforeFile, afterFile, onRefresh,
}: ChangeDetectionTabProps) {
  const { data, loading, error, analyze } = useAnalyze(settings, ready);
  const [autoAlign, setAutoAlign] = useState(true);
  const [drawing, setDrawing] = useState(false);
  const [selectedRank, setSelectedRank] = useState<number | null>(null);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  useEffect(() => {
    const timer = setTimeout(analyze, 300);
    return () => clearTimeout(timer);
  }, [ready, imageKey, settings, analyze]);

  // A stale selection from a previous analysis must not linger.
  useEffect(() => { setSelectedRank(null); }, [imageKey]);

  const highlightUrl = data?.images?.highlight;
  const compare = useMemo(() => [{ label: 'Before', src: beforeUrl }, { label: 'After', src: afterUrl }], [beforeUrl, afterUrl]);

  const changeAutoAlign = async (v: boolean) => {
    setAutoAlign(v);
    try {
      const fd = new FormData();
      fd.append('auto', v ? '1' : '0');
      await postForm('/api/registration', fd);
      onRefresh();
    } catch {
      setAutoAlign(!v);
    }
  };

  const exportReport = async () => {
    if (!data || !highlightUrl || !beforeFile || !afterFile) return;
    setExporting(true);
    setExportError(null);
    try {
      const [beforeHash, afterHash, beforeImg, afterImg, hl] = await Promise.all([
        sha256Hex(beforeFile), sha256Hex(afterFile),
        toJpegDataUrl(beforeUrl), toJpegDataUrl(afterUrl), toJpegDataUrl(highlightUrl),
      ]);
      const crops: Record<number, string> = {};
      await Promise.all(data.regions.slice(0, 20).map(async r => { crops[r.rank] = await cropDataUrl(afterUrl, r); }));
      const html = buildReportHtml({
        generatedAt: new Date(),
        before: { name: beforeFile.name, sha256: beforeHash, dataUrl: beforeImg },
        after: { name: afterFile.name, sha256: afterHash, dataUrl: afterImg },
        highlightDataUrl: hl, analysis: data, settings, crops,
      });
      const a = document.createElement('a');
      a.href = URL.createObjectURL(new Blob([html], { type: 'text/html' }));
      a.download = `tscm-report-${new Date().toISOString().replace(/[:.]/g, '-')}.html`;
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 1000);
    } catch (e) {
      setExportError((e as Error).message);
    } finally {
      setExporting(false);
    }
  };

  const reg = data?.registration;
  const regSeverity = reg?.mode === 'auto' ? 'success' : reg?.message.startsWith('rejected') ? 'warning' : 'info';

  return (
    <Box>
      {/* Primary controls */}
      <Box sx={{ display: 'flex', gap: 3, mb: 2, flexWrap: 'wrap', alignItems: 'flex-end' }}>
        <Labeled label={`Detection Strength: ${settings.strength}${settings.adaptiveThreshold ? ' (adaptive)' : ''}`}>
          <Slider
            aria-label="Detection strength" value={settings.strength} min={5} max={100} step={1} size="small"
            onChange={(_, v) => onSettings({ strength: v as number })}
          />
        </Labeled>
        <Labeled label={`Noise Reduction: ${settings.morphSize <= 1 ? 'off' : `${settings.morphSize}×${settings.morphSize}`}`}>
          <Slider
            aria-label="Noise reduction" value={settings.morphSize} min={1} max={15} step={1} size="small"
            onChange={(_, v) => onSettings({ morphSize: v as number })}
          />
        </Labeled>
        <Labeled label="Highlight Color">
          <Box role="radiogroup" aria-label="Highlight color" sx={{ display: 'flex', gap: 1, pb: '9px', alignItems: 'center' }}>
            {HIGHLIGHT_PRESETS.map(p => {
              const on = settings.highlightColor === p.hex;
              return (
                <Tooltip key={p.hex} title={p.label} placement="top">
                  <Box
                    component="button"
                    role="radio"
                    aria-checked={on}
                    aria-label={p.label}
                    onClick={() => onSettings({ highlightColor: p.hex })}
                    sx={{
                      width: 28, height: 28, borderRadius: '50%', background: p.hex,
                      border: on ? '3px solid white' : '2px solid transparent',
                      outline: on ? '2px solid rgba(255,255,255,0.4)' : '2px solid rgba(255,255,255,0.1)',
                      cursor: 'pointer', padding: 0, flexShrink: 0, transition: 'transform 0.1s',
                      '&:hover': { transform: 'scale(1.15)' }, '&:focus-visible': { outline: '3px solid #5c9ced' },
                    }}
                  />
                </Tooltip>
              );
            })}
          </Box>
        </Labeled>
        <Labeled label={`Highlight Opacity: ${settings.highlightAlpha}%`}>
          <Slider
            aria-label="Highlight opacity" value={settings.highlightAlpha} min={10} max={100} step={5} size="small"
            onChange={(_, v) => onSettings({ highlightAlpha: v as number })}
          />
        </Labeled>
      </Box>

      <Box sx={{ display: 'flex', gap: 2, mb: 2, flexWrap: 'wrap', alignItems: 'center' }}>
        <Toggle
          label="Auto-align photos" checked={autoAlign} onChange={changeAutoAlign}
          hint="Match features between the photos and correct camera shift/angle before comparing"
        />
        <Toggle
          label="Adaptive threshold" checked={settings.adaptiveThreshold}
          onChange={v => onSettings({ adaptiveThreshold: v })}
          hint="Set the threshold from each image pair's own noise level; Strength then only tunes it"
        />
        <Button
          size="small" variant={drawing ? 'contained' : 'outlined'} color="warning"
          onClick={() => setDrawing(d => !d)} aria-pressed={drawing}
        >
          {drawing ? 'Done marking' : 'Mark ignore zones'}
        </Button>
        {settings.ignore.length > 0 && (
          <Chip size="small" label={`${settings.ignore.length} ignored`} onDelete={() => onSettings({ ignore: [] })} />
        )}
        <Button
          size="small" variant="outlined" startIcon={<DownloadRoundedIcon />}
          disabled={!data || exporting || !beforeFile || !afterFile} onClick={exportReport} sx={{ ml: 'auto' }}
        >
          {exporting ? 'Building…' : 'Export report'}
        </Button>
      </Box>

      {drawing && <Alert severity="info" sx={{ mb: 2 }}>Drag on the image to mark areas to ignore (screens, windows, anything that legitimately changes).</Alert>}
      {reg && reg.message && <Alert severity={regSeverity} sx={{ mb: 2 }}>Alignment: {reg.message}{data && data.baselines > 1 ? ` · ${data.baselines} baselines combined` : ''}</Alert>}
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      {exportError && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setExportError(null)}>Report failed: {exportError}</Alert>}

      {/* Result image */}
      <Box sx={{ mb: 3, display: 'flex', justifyContent: 'center', flexDirection: 'column', alignItems: 'center', gap: 1 }}>
        {loading && <LinearProgress sx={{ width: '100%' }} aria-label="Analyzing" />}
        {highlightUrl ? (
          <ResultImage
            src={highlightUrl}
            caption="Changes Highlighted on After"
            compare={compare}
            overlay={
              <ImageOverlay
                regions={data?.regions}
                selectedRank={selectedRank}
                onSelectRank={setSelectedRank}
                ignore={settings.ignore}
                onIgnoreChange={ignore => onSettings({ ignore })}
                drawing={drawing}
              />
            }
          />
        ) : (
          !error && ready && <Skeleton variant="rounded" width="70%" height={360} animation="wave" />
        )}
      </Box>

      {data && highlightUrl && (
        <Box sx={{ mb: 3 }}>
          <RegionsPanel
            regions={data.regions}
            total={data.stats.regions}
            selectedRank={selectedRank}
            onSelect={setSelectedRank}
            afterUrl={afterUrl}
            highlightUrl={highlightUrl}
          />
        </Box>
      )}

      {/* Advanced section */}
      {data && (
        <Accordion
          disableGutters elevation={0}
          sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '8px !important', '&:before': { display: 'none' } }}
        >
          <AccordionSummary expandIcon={<ExpandMoreIcon />}>
            <Typography variant="body2" color="text.secondary">Advanced Options &amp; Stats</Typography>
          </AccordionSummary>
          <AccordionDetails>
            <Box sx={{ display: 'flex', gap: 3, mb: 2, flexWrap: 'wrap', alignItems: 'flex-end' }}>
              <Labeled label={`Min Region Size: ${settings.minRegion} px`}>
                <Slider aria-label="Minimum region size" value={settings.minRegion} min={1} max={500} step={1} size="small"
                  onChange={(_, v) => onSettings({ minRegion: v as number })} />
              </Labeled>
              <Labeled label={`Pre-blur: ${settings.preBlurSigma === 0 ? 'off' : `σ=${settings.preBlurSigma}`}`}>
                <Slider aria-label="Pre-blur" value={settings.preBlurSigma} min={0} max={4} step={0.5} size="small"
                  onChange={(_, v) => onSettings({ preBlurSigma: v as number })} />
              </Labeled>
              <Labeled label={`Fill Gaps: ${settings.closeSize <= 1 ? 'off' : `${settings.closeSize}×${settings.closeSize}`}`}>
                <Slider aria-label="Fill gaps" value={settings.closeSize} min={1} max={15} step={1} size="small"
                  onChange={(_, v) => onSettings({ closeSize: v as number })} />
              </Labeled>
              <Labeled label={`Shift Tolerance: ${settings.shiftTolerance === 0 ? 'off' : `±${settings.shiftTolerance} px`}`}>
                <Slider aria-label="Shift tolerance" value={settings.shiftTolerance} min={0} max={3} step={1} size="small"
                  onChange={(_, v) => onSettings({ shiftTolerance: v as number })} />
              </Labeled>
            </Box>
            <Box sx={{ display: 'flex', gap: 2, mb: 2, flexWrap: 'wrap' }}>
              <Toggle label="Match exposure" checked={settings.matchIntensity} onChange={v => onSettings({ matchIntensity: v })}
                hint="Fit per-channel gain/offset so brightness or white-balance drift isn't flagged" />
              <Toggle label="Normalize lighting" checked={settings.normalizeLuma} onChange={v => onSettings({ normalizeLuma: v })}
                hint="Simple mean-brightness shift (used when Match exposure is off)" />
              <Toggle label="Colour-aware" checked={settings.colorAware} onChange={v => onSettings({ colorAware: v })}
                hint="Also compare colour, not just brightness, so colour-only changes are caught" />
            </Box>

            <Divider sx={{ mb: 2 }} />
            <StatsBar stats={data.stats} />
            <Typography variant="caption" color="text.secondary">
              Threshold used: {data.threshold} · analysed at {data.analysis_dims.w}×{data.analysis_dims.h}
            </Typography>
          </AccordionDetails>
        </Accordion>
      )}
    </Box>
  );
}
