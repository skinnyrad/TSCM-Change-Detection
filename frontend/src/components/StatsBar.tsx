import Box from '@mui/material/Box';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';
import { Gauge } from '@mui/x-charts/Gauge';
import type { AnalyzeStats } from '../types/api';

interface StatsBarProps {
  stats: AnalyzeStats;
}

export function StatsBar({ stats }: StatsBarProps) {
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 2, mb: 3 }}>
      {/* Gauge for % changed */}
      <Paper variant="outlined" sx={{ p: 2, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
        <Gauge
          value={stats.pct}
          valueMin={0}
          valueMax={100}
          width={120}
          height={80}
          text={({ value }) => `${value}%`}
          sx={{ '& .MuiGauge-valueText': { fontSize: 14, fontWeight: 700 } }}
        />
        <Typography variant="caption" color="text.secondary" mt={0.5}>
          Changed Area
        </Typography>
      </Paper>

      {/* Changed pixels */}
      <Paper variant="outlined" sx={{ p: 2, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
        <Typography variant="h5" fontWeight={700} color="primary">
          {stats.changed_px.toLocaleString()}
        </Typography>
        <Typography variant="caption" color="text.secondary" mt={0.5}>
          Changed Pixels
        </Typography>
      </Paper>

      {/* Distinct regions */}
      <Paper variant="outlined" sx={{ p: 2, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
        <Typography variant="h5" fontWeight={700} color="primary">
          {stats.regions}
        </Typography>
        <Typography variant="caption" color="text.secondary" mt={0.5}>
          Distinct Regions
        </Typography>
      </Paper>
    </Box>
  );
}
