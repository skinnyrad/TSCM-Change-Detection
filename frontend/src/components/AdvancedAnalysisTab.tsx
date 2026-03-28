import { useEffect, useState } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Divider from '@mui/material/Divider';
import Slider from '@mui/material/Slider';
import Typography from '@mui/material/Typography';
import { useAnalyze } from '../hooks/useAnalyze';
import { ResultImage } from './ResultImage';
import { StatsBar } from './StatsBar';

interface AdvancedAnalysisTabProps {
  ready: boolean;
  imageKey: string;
}

export function AdvancedAnalysisTab({ ready, imageKey }: AdvancedAnalysisTabProps) {
  const [strength, setStrength] = useState(40);
  const [morphSize, setMorphSize] = useState(2);
  const [minRegion, setMinRegion] = useState(50);
  const [cannyLow, setCannyLow] = useState(100);
  const [cannyHigh, setCannyHigh] = useState(200);

  const cannyInvalid = cannyLow >= cannyHigh;

  const { data, error, analyze } = useAnalyze({
    method: 'advanced',
    strength,
    minRegion,
    morphSize,
    cannyLow,
    cannyHigh,
    ready,
  });

  useEffect(() => {
    if (cannyInvalid) return;
    const timer = setTimeout(analyze, 300);
    return () => clearTimeout(timer);
  }, [ready, imageKey, strength, morphSize, minRegion, cannyLow, cannyHigh, cannyInvalid, analyze]);

  const images = data?.images;

  return (
    <Box>
      {/* Controls */}
      <Box sx={{ display: 'flex', gap: 3, mb: 3, flexWrap: 'wrap' }}>
        <Box sx={{ flex: 1, minWidth: 120 }}>
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
        <Box sx={{ flex: 1, minWidth: 120 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Noise Reduction: {morphSize <= 1 ? 'off' : `${morphSize}×${morphSize}`}
          </Typography>
          <Slider
            value={morphSize}
            min={1}
            max={11}
            step={1}
            marks
            onChange={(_, v) => setMorphSize(v as number)}
            size="small"
          />
        </Box>
        <Box sx={{ flex: 1, minWidth: 120 }}>
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
        <Box sx={{ flex: 1, minWidth: 120 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Canny Low: {cannyLow}
          </Typography>
          <Slider
            value={cannyLow}
            min={1}
            max={254}
            step={1}
            onChange={(_, v) => setCannyLow(v as number)}
            size="small"
            color={cannyInvalid ? 'error' : 'primary'}
          />
        </Box>
        <Box sx={{ flex: 1, minWidth: 120 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Canny High: {cannyHigh}
          </Typography>
          <Slider
            value={cannyHigh}
            min={2}
            max={255}
            step={1}
            onChange={(_, v) => setCannyHigh(v as number)}
            size="small"
            color={cannyInvalid ? 'error' : 'primary'}
          />
        </Box>
      </Box>

      {cannyInvalid && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          Canny Low Threshold must be less than High Threshold.
        </Alert>
      )}

      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      {data && (
        <>
          <StatsBar stats={data.stats} />
          <Divider sx={{ mb: 3 }} />

          <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))' }}>
            {images?.edges && (
              <ResultImage src={images.edges} caption="Edge Detection on Diff" />
            )}
            {images?.contours && (
              <ResultImage
                src={images.contours}
                caption={`Contours on After Image (${data.stats.regions} region${data.stats.regions !== 1 ? 's' : ''} detected)`}
              />
            )}
          </Box>
        </>
      )}
    </Box>
  );
}
