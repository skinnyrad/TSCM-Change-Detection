import { useEffect, useMemo } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import LinearProgress from '@mui/material/LinearProgress';
import Skeleton from '@mui/material/Skeleton';
import { useAltAnalysis } from '../hooks/useAltAnalysis';
import type { DetectionSettings } from '../lib/settings';
import { ResultImage, type Frame } from './ResultImage';

interface AlternateAnalysisTabProps {
  ready: boolean;
  imageKey: string;
  settings: DetectionSettings;
}

const VIEWS: [key: 'diff' | 'subtraction' | 'heatmap' | 'edges', caption: string][] = [
  ['diff', 'Image Difference'],
  ['subtraction', 'Channel Subtraction'],
  ['heatmap', 'Change Intensity Heatmap'],
  ['edges', 'Canny Edge Detection'],
];

export function AlternateAnalysisTab({ ready, imageKey, settings }: AlternateAnalysisTabProps) {
  const { data, loading, error, analyze } = useAltAnalysis(settings, ready);

  useEffect(() => {
    const timer = setTimeout(analyze, 300);
    return () => clearTimeout(timer);
  }, [ready, imageKey, settings, analyze]);

  // In fullscreen, ←/→ flips through every alternate view of the same pair.
  const frames = useMemo<Frame[]>(
    () => (data ? VIEWS.flatMap(([k, caption]) => (data[k] ? [{ label: caption, src: data[k] as string }] : [])) : []),
    [data],
  );

  return (
    <Box>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      {loading && <LinearProgress sx={{ mb: 2 }} aria-label="Analyzing" />}
      {!data && !error && ready && (
        <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))' }}>
          {VIEWS.map(([k]) => <Skeleton key={k} variant="rounded" height={240} animation="wave" />)}
        </Box>
      )}
      {data && (
        <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))' }}>
          {VIEWS.map(([k, caption]) => {
            const src = data[k];
            return src ? (
              <ResultImage key={k} src={src} caption={caption} compare={frames.filter(f => f.src !== src)} />
            ) : null;
          })}
        </Box>
      )}
    </Box>
  );
}
