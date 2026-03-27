import { useRef } from 'react';
import Box from '@mui/material/Box';
import IconButton from '@mui/material/IconButton';
import Paper from '@mui/material/Paper';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import FullscreenRoundedIcon from '@mui/icons-material/FullscreenRounded';

interface ResultImageProps {
  src: string;
  caption: string;
}

export function ResultImage({ src, caption }: ResultImageProps) {
  const imgRef = useRef<HTMLImageElement>(null);

  return (
    <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
      <Box
        sx={{
          position: 'relative',
          '&:hover .fs-btn': { opacity: 1 },
        }}
      >
        <Box
          ref={imgRef}
          component="img"
          src={src}
          alt={caption}
          sx={{ width: '100%', display: 'block', objectFit: 'contain' }}
        />
        <Tooltip title="Fullscreen" placement="left">
          <IconButton
            className="fs-btn"
            size="small"
            onClick={() => imgRef.current?.requestFullscreen()}
            sx={{
              position: 'absolute',
              top: 8,
              right: 8,
              opacity: 0,
              transition: 'opacity 0.15s',
              bgcolor: 'rgba(0,0,0,0.55)',
              '&:hover': { bgcolor: 'rgba(0,0,0,0.8)' },
            }}
          >
            <FullscreenRoundedIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>
      <Typography
        variant="caption"
        display="block"
        textAlign="center"
        sx={{ py: 0.75, px: 1, color: 'text.secondary' }}
      >
        {caption}
      </Typography>
    </Paper>
  );
}
