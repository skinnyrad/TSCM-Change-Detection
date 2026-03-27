import { useEffect, useState } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Divider from '@mui/material/Divider';
import Grid from '@mui/material/Grid';
import Slider from '@mui/material/Slider';
import Typography from '@mui/material/Typography';
import { useAnalyze } from '../hooks/useAnalyze';
import { ResultImage } from './ResultImage';
import { StatsBar } from './StatsBar';

interface AdvancedAnalysisTabProps {
  before: File;
  after: File;
}

export function AdvancedAnalysisTab({ before, after }: AdvancedAnalysisTabProps) {
  const [sensitivity, setSensitivity] = useState(30);
  const [cannyLow, setCannyLow] = useState(100);
  const [cannyHigh, setCannyHigh] = useState(200);

  const cannyInvalid = cannyLow >= cannyHigh;

  const { data, error, analyze } = useAnalyze({
    before,
    after,
    method: 'advanced',
    sensitivity,
    cannyLow,
    cannyHigh,
  });

  useEffect(() => {
    if (cannyInvalid) return;
    const timer = setTimeout(analyze, 300);
    return () => clearTimeout(timer);
  }, [before, after, sensitivity, cannyLow, cannyHigh, cannyInvalid, analyze]);

  const images = data?.images;

  return (
    <Box>
      {/* Controls */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid size={{ xs: 12, sm: 4 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Detection Threshold: {sensitivity}
          </Typography>
          <Slider
            value={sensitivity}
            min={5}
            max={100}
            step={1}
            onChange={(_, v) => setSensitivity(v as number)}
            size="small"
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 4 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Canny Low Threshold: {cannyLow}
          </Typography>
          <Slider
            value={cannyLow}
            min={20}
            max={150}
            step={1}
            onChange={(_, v) => setCannyLow(v as number)}
            size="small"
            color={cannyInvalid ? 'error' : 'primary'}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 4 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Canny High Threshold: {cannyHigh}
          </Typography>
          <Slider
            value={cannyHigh}
            min={50}
            max={300}
            step={1}
            onChange={(_, v) => setCannyHigh(v as number)}
            size="small"
            color={cannyInvalid ? 'error' : 'primary'}
          />
        </Grid>
      </Grid>

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
