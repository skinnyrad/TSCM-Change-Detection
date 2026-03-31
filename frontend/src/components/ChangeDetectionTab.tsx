import { useEffect, useState } from 'react';
import Accordion from '@mui/material/Accordion';
import AccordionDetails from '@mui/material/AccordionDetails';
import AccordionSummary from '@mui/material/AccordionSummary';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Divider from '@mui/material/Divider';
import FormControlLabel from '@mui/material/FormControlLabel';
import Slider from '@mui/material/Slider';
import Switch from '@mui/material/Switch';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import { useAnalyze } from '../hooks/useAnalyze';
import { ResultImage } from './ResultImage';
import { StatsBar } from './StatsBar';

interface ChangeDetectionTabProps {
  ready: boolean;
  imageKey: string;
}

const HIGHLIGHT_PRESETS = [
  { label: 'Red',     hex: '#ff3c3c' },
  { label: 'Orange',  hex: '#ff8c00' },
  { label: 'Yellow',  hex: '#ffd700' },
  { label: 'Cyan',    hex: '#00e5ff' },
  { label: 'Lime',    hex: '#39ff14' },
];

export function ChangeDetectionTab({ ready, imageKey }: ChangeDetectionTabProps) {
  // Primary controls (always visible)
  const [strength, setStrength] = useState(75);
  const [morphSize, setMorphSize] = useState(7);
  const [highlightColor, setHighlightColor] = useState(HIGHLIGHT_PRESETS[0].hex);
  const [highlightAlpha, setHighlightAlpha] = useState(55); // 0–100 for slider

  // Advanced controls (collapsed by default)
  const [minRegion, setMinRegion] = useState(50);
  const [preBlurSigma, setPreBlurSigma] = useState(2.0);
  const [closeSize, setCloseSize] = useState(5);
  const [normalizeLuma, setNormalizeLuma] = useState(true);

  const { data, error, analyze } = useAnalyze({
    strength,
    minRegion,
    morphSize,
    preBlurSigma,
    normalizeLuma,
    closeSize,
    highlightColor,
    highlightAlpha: highlightAlpha / 100,
    ready,
  });

  useEffect(() => {
    const timer = setTimeout(analyze, 300);
    return () => clearTimeout(timer);
  }, [ready, imageKey, strength, morphSize, highlightColor, highlightAlpha,
      minRegion, preBlurSigma, closeSize, normalizeLuma, analyze]);

  return (
    <Box>
      {/* Primary controls */}
      <Box sx={{ display: 'flex', gap: 3, mb: 3, flexWrap: 'wrap', alignItems: 'flex-end' }}>
        <Box sx={{ flex: 1, minWidth: 160 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Detection Strength: {strength}
          </Typography>
          <Slider
            value={strength}
            min={5}
            max={100}
            step={1}
            onChange={(_, v) => setStrength(v as number)}
            size="small"
          />
        </Box>

        <Box sx={{ flex: 1, minWidth: 160 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Noise Reduction: {morphSize <= 1 ? 'off' : `${morphSize}×${morphSize}`}
          </Typography>
          <Slider
            value={morphSize}
            min={1}
            max={15}
            step={1}
            onChange={(_, v) => setMorphSize(v as number)}
            size="small"
          />
        </Box>

        <Box sx={{ minWidth: 160 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Highlight Color
          </Typography>
          <Box sx={{ display: 'flex', gap: 1, pb: '9px', alignItems: 'center' }}>
            {HIGHLIGHT_PRESETS.map((p) => (
              <Tooltip key={p.hex} title={p.label} placement="top">
                <Box
                  component="button"
                  onClick={() => setHighlightColor(p.hex)}
                  sx={{
                    width: 28,
                    height: 28,
                    borderRadius: '50%',
                    background: p.hex,
                    border: highlightColor === p.hex
                      ? '3px solid white'
                      : '2px solid transparent',
                    outline: highlightColor === p.hex
                      ? '2px solid rgba(255,255,255,0.4)'
                      : '2px solid rgba(255,255,255,0.1)',
                    cursor: 'pointer',
                    padding: 0,
                    flexShrink: 0,
                    transition: 'transform 0.1s',
                    '&:hover': { transform: 'scale(1.15)' },
                  }}
                />
              </Tooltip>
            ))}
          </Box>
        </Box>

        <Box sx={{ flex: 1, minWidth: 160 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Highlight Opacity: {highlightAlpha}%
          </Typography>
          <Slider
            value={highlightAlpha}
            min={10}
            max={100}
            step={5}
            onChange={(_, v) => setHighlightAlpha(v as number)}
            size="small"
          />
        </Box>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* Result image */}
      {data?.images?.highlight && (
        <Box sx={{ mb: 3, display: 'flex', justifyContent: 'center' }}>
          <ResultImage src={data.images.highlight} caption="Changes Highlighted on After" />
        </Box>
      )}

      {/* Advanced section */}
      {data && (
        <Accordion
          disableGutters
          elevation={0}
          sx={{
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: '8px !important',
            '&:before': { display: 'none' },
          }}
        >
          <AccordionSummary expandIcon={<ExpandMoreIcon />}>
            <Typography variant="body2" color="text.secondary">
              Advanced Options &amp; Stats
            </Typography>
          </AccordionSummary>
          <AccordionDetails>
            <Box sx={{ display: 'flex', gap: 3, mb: 3, flexWrap: 'wrap', alignItems: 'flex-end' }}>
              <Box sx={{ flex: 1, minWidth: 160 }}>
                <Typography variant="body2" color="text.secondary" gutterBottom>
                  Min Region Size: {minRegion} px
                </Typography>
                <Slider
                  value={minRegion}
                  min={1}
                  max={500}
                  step={1}
                  onChange={(_, v) => setMinRegion(v as number)}
                  size="small"
                />
              </Box>

              <Box sx={{ flex: 1, minWidth: 160 }}>
                <Typography variant="body2" color="text.secondary" gutterBottom>
                  Pre-blur: {preBlurSigma === 0 ? 'off' : `σ=${preBlurSigma}`}
                </Typography>
                <Slider
                  value={preBlurSigma}
                  min={0}
                  max={4}
                  step={0.5}
                  onChange={(_, v) => setPreBlurSigma(v as number)}
                  size="small"
                />
              </Box>

              <Box sx={{ flex: 1, minWidth: 160 }}>
                <Typography variant="body2" color="text.secondary" gutterBottom>
                  Fill Gaps: {closeSize <= 1 ? 'off' : `${closeSize}×${closeSize}`}
                </Typography>
                <Slider
                  value={closeSize}
                  min={1}
                  max={15}
                  step={1}
                  onChange={(_, v) => setCloseSize(v as number)}
                  size="small"
                />
              </Box>

              <Box sx={{ display: 'flex', alignItems: 'center', pb: 0.5 }}>
                <FormControlLabel
                  control={
                    <Switch
                      checked={normalizeLuma}
                      onChange={(e) => setNormalizeLuma(e.target.checked)}
                      size="small"
                    />
                  }
                  label={
                    <Typography variant="body2" color="text.secondary">
                      Normalize Lighting
                    </Typography>
                  }
                />
              </Box>
            </Box>

            <Divider sx={{ mb: 2 }} />
            <StatsBar stats={data.stats} />
          </AccordionDetails>
        </Accordion>
      )}
    </Box>
  );
}
