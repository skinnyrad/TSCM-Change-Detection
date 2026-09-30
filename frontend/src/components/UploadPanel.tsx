import { useRef, useState } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';
import UploadFileRoundedIcon from '@mui/icons-material/UploadFileRounded';

interface DropZoneProps {
  label: string;
  file: File | null;
  // Server-rendered PNG URL — guaranteed displayable regardless of input format.
  displayUrl?: string | null;
  onFile: (f: File) => void;
}

function DropZone({ label, file, displayUrl, onFile }: DropZoneProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragOver, setDragOver] = useState(false);
  const open = () => inputRef.current?.click();

  return (
    <Paper
      variant="outlined"
      role="button"
      tabIndex={0}
      aria-label={file ? `${label}: ${file.name}. Press Enter to replace.` : `${label}. Press Enter to choose a file.`}
      onClick={open}
      onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); open(); } }}
      onDragOver={e => { e.preventDefault(); setDragOver(true); }}
      onDragLeave={() => setDragOver(false)}
      onDrop={e => {
        e.preventDefault();
        setDragOver(false);
        const f = e.dataTransfer.files?.[0];
        if (f) onFile(f);
      }}
      sx={{
        flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1.5, p: 2, cursor: 'pointer',
        transition: 'border-color 0.2s', borderColor: dragOver ? 'primary.main' : undefined,
        '&:hover': { borderColor: 'primary.main' }, '&:focus-visible': { outline: '3px solid #5c9ced', outlineOffset: 2 },
        minHeight: 200, justifyContent: file ? 'flex-start' : 'center',
      }}
    >
      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png"
        aria-hidden
        tabIndex={-1}
        style={{ display: 'none' }}
        onChange={e => {
          const f = e.target.files?.[0];
          if (f) onFile(f);
          e.target.value = ''; // allow re-selecting the same file
        }}
      />
      {file ? (
        <>
          {displayUrl && (
            <Box component="img" src={displayUrl} alt={label} sx={{ width: '100%', maxHeight: 300, objectFit: 'contain', borderRadius: 1 }} />
          )}
          <Typography variant="caption" color="text.secondary" textAlign="center">
            {file.name} · Click or drop to replace
          </Typography>
        </>
      ) : (
        <>
          <UploadFileRoundedIcon sx={{ fontSize: 48, color: 'text.disabled' }} />
          <Typography variant="body1" color="text.secondary">{label}</Typography>
          <Button variant="outlined" size="small" component="span" tabIndex={-1}>Choose Image</Button>
          <Typography variant="caption" color="text.disabled">JPG or PNG — or drop a file here</Typography>
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
  onBaseline: (f: File) => void;
  onClearBaselines: () => void;
  onResetAlignment: () => void;
  beforeDisplayUrl?: string | null;
  afterDisplayUrl?: string | null;
  baselines: number;
  uploadingBaseline: boolean;
  alignmentActive?: boolean;
  error?: string | null;
}

export function UploadPanel({
  before, after, onBefore, onAfter, onBaseline, onClearBaselines, onResetAlignment,
  beforeDisplayUrl, afterDisplayUrl, baselines, uploadingBaseline, alignmentActive, error,
}: UploadPanelProps) {
  const baselineInput = useRef<HTMLInputElement>(null);
  return (
    <Box sx={{ mb: 3 }}>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      <Box sx={{ display: 'flex', gap: 2 }}>
        <DropZone label="Before Image" file={before} displayUrl={beforeDisplayUrl} onFile={onBefore} />
        <DropZone label="After Image" file={after} displayUrl={afterDisplayUrl} onFile={onAfter} />
      </Box>
      {before && after && (
        <Box sx={{ display: 'flex', gap: 1.5, mt: 1.5, alignItems: 'center', flexWrap: 'wrap' }}>
          <input
            ref={baselineInput} type="file" accept="image/jpeg,image/png" hidden aria-hidden tabIndex={-1}
            onChange={e => {
              const f = e.target.files?.[0];
              if (f) onBaseline(f);
              e.target.value = '';
            }}
          />
          <Button size="small" variant="outlined" disabled={uploadingBaseline} onClick={() => baselineInput.current?.click()}>
            Add extra baseline photo
          </Button>
          {baselines > 0 && (
            <Chip size="small" color="primary" label={`${baselines + 1} baselines combined`} onDelete={onClearBaselines} />
          )}
          <Typography variant="caption" color="text.disabled">
            Extra &ldquo;Before&rdquo; shots of the same spot teach the tool what normally varies, cutting false positives.
          </Typography>
          {alignmentActive && (
            <Button size="small" color="warning" onClick={onResetAlignment} sx={{ ml: 'auto' }}>
              Reset manual alignment
            </Button>
          )}
        </Box>
      )}
    </Box>
  );
}
