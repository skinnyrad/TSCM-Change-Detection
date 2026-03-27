import { useEffect, useRef, useState } from 'react';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import ButtonGroup from '@mui/material/ButtonGroup';
import ToggleButton from '@mui/material/ToggleButton';
import ToggleButtonGroup from '@mui/material/ToggleButtonGroup';
import Typography from '@mui/material/Typography';
import {
  ReactCompareSlider,
  ReactCompareSliderImage,
} from 'react-compare-slider';

interface ImageComparisonTabProps {
  before: File;
  after: File;
}

export function ImageComparisonTab({ before, after }: ImageComparisonTabProps) {
  const [mode, setMode] = useState<'slider' | 'toggle'>('slider');
  const [showAfter, setShowAfter] = useState(false);

  const beforeUrl = useRef('');
  const afterUrl = useRef('');

  // Create object URLs for the files
  beforeUrl.current = URL.createObjectURL(before);
  afterUrl.current = URL.createObjectURL(after);

  useEffect(() => {
    const bUrl = URL.createObjectURL(before);
    const aUrl = URL.createObjectURL(after);
    beforeUrl.current = bUrl;
    afterUrl.current = aUrl;
    return () => {
      URL.revokeObjectURL(bUrl);
      URL.revokeObjectURL(aUrl);
    };
  }, [before, after]);

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        <Typography variant="body2" color="text.secondary">
          Comparison Mode:
        </Typography>
        <ToggleButtonGroup
          value={mode}
          exclusive
          onChange={(_, v) => { if (v) setMode(v); }}
          size="small"
        >
          <ToggleButton value="slider">Slider</ToggleButton>
          <ToggleButton value="toggle">Toggle</ToggleButton>
        </ToggleButtonGroup>
      </Box>

      {mode === 'slider' ? (
        <ReactCompareSlider
          itemOne={<ReactCompareSliderImage src={beforeUrl.current} alt="Before" />}
          itemTwo={<ReactCompareSliderImage src={afterUrl.current} alt="After" />}
          style={{ borderRadius: 4, overflow: 'hidden' }}
        />
      ) : (
        <Box>
          <ButtonGroup variant="outlined" sx={{ mb: 2 }}>
            <Button onClick={() => setShowAfter(false)} variant={!showAfter ? 'contained' : 'outlined'}>
              Before
            </Button>
            <Button onClick={() => setShowAfter(true)} variant={showAfter ? 'contained' : 'outlined'}>
              After
            </Button>
            <Button onClick={() => setShowAfter(v => !v)}>↔ Toggle</Button>
          </ButtonGroup>
          <Box>
            <Box
              component="img"
              src={showAfter ? afterUrl.current : beforeUrl.current}
              alt={showAfter ? 'After' : 'Before'}
              sx={{ width: '100%', borderRadius: 1, display: 'block' }}
            />
            <Typography variant="caption" color="text.secondary" mt={0.5} display="block" textAlign="center">
              {showAfter ? 'After' : 'Before'}
            </Typography>
          </Box>
        </Box>
      )}
    </Box>
  );
}
