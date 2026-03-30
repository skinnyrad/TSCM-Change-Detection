import { useEffect } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import { useAltAnalysis } from '../hooks/useAltAnalysis';
import { ResultImage } from './ResultImage';

interface AlternateAnalysisTabProps {
  ready: boolean;
  imageKey: string;
}

export function AlternateAnalysisTab({ ready, imageKey }: AlternateAnalysisTabProps) {
  const { data, error, analyze } = useAltAnalysis({
    strength: 65,
    morphSize: 5,
    minRegion: 25,
    ready,
  });

  useEffect(() => {
    const timer = setTimeout(analyze, 300);
    return () => clearTimeout(timer);
  }, [ready, imageKey, analyze]);

  return (
    <Box>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      {data && (
        <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))' }}>
          {data.diff && (
            <ResultImage src={data.diff} caption="Image Difference" />
          )}
          {data.subtraction && (
            <ResultImage src={data.subtraction} caption="Channel Subtraction" />
          )}
          {data.heatmap && (
            <ResultImage src={data.heatmap} caption="Change Intensity Heatmap" />
          )}
          {data.edges && (
            <ResultImage src={data.edges} caption="Canny Edge Detection" />
          )}
        </Box>
      )}
    </Box>
  );
}
