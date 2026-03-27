import { useRef } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';
import UploadFileRoundedIcon from '@mui/icons-material/UploadFileRounded';

interface DropZoneProps {
  label: string;
  file: File | null;
  onFile: (f: File) => void;
}

function DropZone({ label, file, onFile }: DropZoneProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const objectUrl = file ? URL.createObjectURL(file) : null;

  return (
    <Paper
      variant="outlined"
      sx={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 1.5,
        p: 2,
        cursor: 'pointer',
        transition: 'border-color 0.2s',
        '&:hover': { borderColor: 'primary.main' },
        minHeight: 200,
        justifyContent: file ? 'flex-start' : 'center',
      }}
      onClick={() => inputRef.current?.click()}
    >
      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png"
        style={{ display: 'none' }}
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) onFile(f);
        }}
      />

      {file && objectUrl ? (
        <>
          <Box
            component="img"
            src={objectUrl}
            alt={label}
            sx={{ width: '100%', maxHeight: 300, objectFit: 'contain', borderRadius: 1 }}
            onLoad={() => URL.revokeObjectURL(objectUrl)}
          />
          <Typography variant="caption" color="text.secondary" textAlign="center">
            {file.name} · Click to replace
          </Typography>
        </>
      ) : (
        <>
          <UploadFileRoundedIcon sx={{ fontSize: 48, color: 'text.disabled' }} />
          <Typography variant="body1" color="text.secondary">
            {label}
          </Typography>
          <Button variant="outlined" size="small" component="span">
            Choose Image
          </Button>
          <Typography variant="caption" color="text.disabled">
            JPG or PNG
          </Typography>
        </>
      )}
    </Paper>
  );
}

interface UploadPanelProps {
  before: File | null;
  after: File | null;
  onBefore: (f: File) => void;
  onAfter: (f: File) => void;
  resized?: boolean;
  beforeDims?: { w: number; h: number };
  afterDims?: { w: number; h: number };
}

export function UploadPanel({ before, after, onBefore, onAfter, resized, beforeDims, afterDims }: UploadPanelProps) {
  return (
    <Box sx={{ mb: 3 }}>
      <Box sx={{ display: 'flex', gap: 2, mb: resized ? 2 : 0 }}>
        <DropZone label="Before Image" file={before} onFile={onBefore} />
        <DropZone label="After Image" file={after} onFile={onAfter} />
      </Box>
      {resized && beforeDims && afterDims && (
        <Alert severity="warning" sx={{ mt: 2 }}>
          Images have different sizes ({beforeDims.w}×{beforeDims.h} vs {afterDims.w}×{afterDims.h}). The Before image was automatically resized to match the After image for analysis.
        </Alert>
      )}
    </Box>
  );
}
