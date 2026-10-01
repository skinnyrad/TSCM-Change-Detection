import { useEffect, useState } from 'react';
import Box from '@mui/material/Box';
import LinearProgress from '@mui/material/LinearProgress';
import List from '@mui/material/List';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemText from '@mui/material/ListItemText';
import Typography from '@mui/material/Typography';
import { cropDataUrl } from '../lib/images';
import type { Region } from '../types/api';

interface RegionsPanelProps {
  regions: Region[];
  total: number;
  selectedRank: number | null;
  onSelect: (rank: number | null) => void;
  afterUrl: string;
  highlightUrl: string;
}

/** Ranked findings, strongest first; selecting one zooms into a crop of After and the highlight. */
export function RegionsPanel({ regions, total, selectedRank, onSelect, afterUrl, highlightUrl }: RegionsPanelProps) {
  const selected = regions.find(r => r.rank === selectedRank) ?? null;
  const [crops, setCrops] = useState<{ after: string; highlight: string } | null>(null);

  useEffect(() => {
    let live = true;
    setCrops(null);
    if (!selected) return;
    Promise.all([cropDataUrl(afterUrl, selected), cropDataUrl(highlightUrl, selected)])
      .then(([after, highlight]) => { if (live) setCrops({ after, highlight }); })
      .catch(() => {});
    return () => { live = false; };
  }, [selected, afterUrl, highlightUrl]);

  if (regions.length === 0) {
    return <Typography variant="body2" color="text.secondary">No regions detected at these settings.</Typography>;
  }
  return (
    <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
      <Box sx={{ flex: '1 1 280px', minWidth: 260 }}>
        <Typography variant="subtitle2" gutterBottom>
          Findings ({total > regions.length ? `top ${regions.length} of ${total}` : total}) — strongest first
        </Typography>
        <List dense disablePadding sx={{ maxHeight: 320, overflow: 'auto', border: 1, borderColor: 'divider', borderRadius: 1 }}>
          {regions.map(r => (
            <ListItemButton key={r.rank} selected={r.rank === selectedRank} onClick={() => onSelect(r.rank === selectedRank ? null : r.rank)}>
              <ListItemText
                primary={`#${r.rank} · ${r.area_pct.toFixed(2)}% of image`}
                secondary={
                  <Box component="span" sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <LinearProgress
                      variant="determinate" value={Math.round(r.score * 100)} aria-label={`Relative strength ${Math.round(r.score * 100)}%`}
                      sx={{ flex: 1, height: 6, borderRadius: 3 }}
                    />
                    <span>{Math.round(r.score * 100)}%</span>
                  </Box>
                }
                slotProps={{ secondary: { component: 'div' } }}
              />
            </ListItemButton>
          ))}
        </List>
      </Box>
      {selected && (
        <Box sx={{ flex: '1 1 320px', display: 'flex', gap: 1.5, flexWrap: 'wrap', alignItems: 'flex-start' }}>
          {crops ? (
            <>
              <Box><Box component="img" src={crops.after} alt={`Region ${selected.rank} in After`} sx={{ maxWidth: '100%', maxHeight: 260, borderRadius: 1 }} /><Typography variant="caption" display="block" color="text.secondary">After</Typography></Box>
              <Box><Box component="img" src={crops.highlight} alt={`Region ${selected.rank} highlighted`} sx={{ maxWidth: '100%', maxHeight: 260, borderRadius: 1 }} /><Typography variant="caption" display="block" color="text.secondary">Highlighted</Typography></Box>
            </>
          ) : <LinearProgress sx={{ width: '100%' }} />}
        </Box>
      )}
    </Box>
  );
}
