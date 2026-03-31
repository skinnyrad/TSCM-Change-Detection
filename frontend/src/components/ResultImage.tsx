import { useRef, useState } from 'react';
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
  const [aspectRatio, setAspectRatio] = useState<number | null>(null);

  return (
    <Paper
      variant="outlined"
      sx={{
        overflow: 'hidden',
        display: 'inline-flex',
        flexDirection: 'column',
        alignItems: 'center',
        width: 'fit-content',
        maxWidth: 'calc(100% - 24px)',
        minWidth: 0,
      }}
    >
      <Box
        sx={{
          position: 'relative',
          width: 'fit-content',
          maxWidth: '100%',
          '&:hover .fs-btn': { opacity: 1 },
        }}
      >
        <Box
          ref={imgRef}
          component="img"
          src={src}
          alt={caption}
          onLoad={(e) => {
            const { naturalWidth, naturalHeight } = e.currentTarget;
            if (naturalWidth > 0 && naturalHeight > 0) {
              setAspectRatio(naturalWidth / naturalHeight);
            }
          }}
          sx={{
            width: aspectRatio ? `min(calc(90vh * ${aspectRatio}), 100%)` : '100%',
            maxWidth: '100%',
            height: 'auto',
            maxHeight: '90vh',
            display: 'block',
            '&:fullscreen, &:-webkit-full-screen': {
              width: 'auto',
              height: 'auto',
              maxWidth: '100vw',
              maxHeight: '90vh',
              margin: 'auto',
            },
          }}
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
        sx={{ py: 0.75, px: 1, color: 'text.secondary', width: '100%', overflowWrap: 'anywhere' }}
      >
        {caption}
      </Typography>
    </Paper>
  );
}
