import { useState } from 'react';
import AppBar from '@mui/material/AppBar';
import Box from '@mui/material/Box';
import Container from '@mui/material/Container';
import Fab from '@mui/material/Fab';
import LinearProgress from '@mui/material/LinearProgress';
import Paper from '@mui/material/Paper';
import Tab from '@mui/material/Tab';
import Tabs from '@mui/material/Tabs';
import Toolbar from '@mui/material/Toolbar';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import CompareRoundedIcon from '@mui/icons-material/CompareRounded';
import FlipRoundedIcon from '@mui/icons-material/FlipRounded';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import BiotechRoundedIcon from '@mui/icons-material/BiotechRounded';
import TransformRoundedIcon from '@mui/icons-material/TransformRounded';
import { UploadPanel } from './components/UploadPanel';
import { ImageComparisonTab } from './components/ImageComparisonTab';
import { ChangeDetectionTab } from './components/ChangeDetectionTab';
import { AlternateAnalysisTab } from './components/AlternateAnalysisTab';
import { AlignmentDialog } from './components/AlignmentDialog';
import { useUpload } from './hooks/useUpload';

const darkTheme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#5c9ced' },
    background: {
      default: '#0f1117',
      paper: '#1a1d27',
    },
  },
  shape: { borderRadius: 8 },
});

export function App() {
  const [before, setBefore] = useState<File | null>(null);
  const [after, setAfter] = useState<File | null>(null);
  const [warpedUrl, setWarpedUrl] = useState<string | null>(null);
  const [alignDialogOpen, setAlignDialogOpen] = useState(false);
  const [activeTab, setActiveTab] = useState(0);

  const {
    uploadBefore,
    uploadAfter,
    uploadingBefore,
    uploadingAfter,
    beforeDims,
    afterDims,
    beforeDisplayUrl,
    afterDisplayUrl,
    ready,
    imageKey,
  } = useUpload();

  const handleBefore = (f: File) => {
    setBefore(f);
    clearWarp();
    uploadBefore(f);
  };

  const handleAfter = (f: File) => {
    setAfter(f);
    clearWarp();
    uploadAfter(f);
  };

  const clearWarp = () => {
    if (warpedUrl) URL.revokeObjectURL(warpedUrl);
    setWarpedUrl(null);
    fetch('/api/clear-warp', { method: 'POST' }).catch(() => {});
  };

  const handleAligned = (url: string) => {
    if (warpedUrl) URL.revokeObjectURL(warpedUrl);
    setWarpedUrl(url);
  };

  const bothSelected = before !== null && after !== null;
  const comparisonBeforeUrl = warpedUrl ?? beforeDisplayUrl ?? '';

  return (
    <ThemeProvider theme={darkTheme}>
      <CssBaseline />
      <AppBar position="static" elevation={0} sx={{ borderBottom: '1px solid', borderColor: 'divider' }}>
        <Toolbar>
          <FlipRoundedIcon sx={{ mr: 1.5 }} />
          <Typography variant="h6" fontWeight={700}>
            TSCM Change Detection
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ ml: 2 }}>
            Upload a Before and After image to identify changes between them.
          </Typography>
        </Toolbar>
      </AppBar>

      {(uploadingBefore || uploadingAfter) && <LinearProgress />}

      <Container maxWidth={false} sx={{ py: 3, maxWidth: '80%', mx: 'auto' }}>
        <UploadPanel
          before={before}
          after={after}
          onBefore={handleBefore}
          onAfter={handleAfter}
          beforeDisplayUrl={warpedUrl ?? beforeDisplayUrl}
          afterDisplayUrl={afterDisplayUrl}
          alignmentActive={warpedUrl !== null}
        />

        {bothSelected && (
          <Paper variant="outlined">
            <Tabs
              value={activeTab}
              onChange={(_, v) => setActiveTab(v)}
              sx={{ borderBottom: '1px solid', borderColor: 'divider', px: 2 }}
            >
              <Tab icon={<CompareRoundedIcon />} iconPosition="start" label="Image Comparison" />
              <Tab icon={<SearchRoundedIcon />} iconPosition="start" label="Change Detection" />
              <Tab icon={<BiotechRoundedIcon />} iconPosition="start" label="Alternate Analysis" />
            </Tabs>

            <Box sx={{ p: 3 }}>
              {activeTab === 0 && beforeDisplayUrl && afterDisplayUrl && (
                <ImageComparisonTab
                  beforeUrl={comparisonBeforeUrl}
                  afterUrl={afterDisplayUrl}
                />
              )}
              {activeTab === 1 && <ChangeDetectionTab ready={ready} imageKey={imageKey} />}
              {activeTab === 2 && <AlternateAnalysisTab ready={ready} imageKey={imageKey} />}
            </Box>
          </Paper>
        )}

        {before && after && beforeDisplayUrl && afterDisplayUrl && beforeDims && afterDims && (
          <AlignmentDialog
            open={alignDialogOpen}
            beforeUrl={beforeDisplayUrl}
            afterUrl={afterDisplayUrl}
            beforeDims={beforeDims}
            afterDims={afterDims}
            onAligned={handleAligned}
            onClose={() => setAlignDialogOpen(false)}
          />
        )}
      </Container>

      {ready && (
        <Tooltip title={warpedUrl ? 'Edit alignment' : 'Align images'} placement="left">
          <Fab
            onClick={() => setAlignDialogOpen(true)}
            sx={{
              position: 'fixed',
              bottom: 32,
              right: 32,
              bgcolor: warpedUrl ? 'primary.main' : 'transparent',
              border: warpedUrl ? 'none' : '2px solid',
              borderColor: 'primary.main',
              color: warpedUrl ? 'primary.contrastText' : 'primary.main',
              '&:hover': {
                bgcolor: warpedUrl ? 'primary.dark' : 'action.hover',
              },
            }}
          >
            <TransformRoundedIcon />
          </Fab>
        </Tooltip>
      )}
      {/* Offscreen keeper images — keep decoded bitmaps alive in the browser's
          paint cache so toggle, alignment dialog, and comparison slider are instant. */}
      {beforeDisplayUrl && (
        <img src={beforeDisplayUrl} alt="" aria-hidden style={{ position: 'fixed', top: '-9999px', left: '-9999px', width: 1, height: 1 }} />
      )}
      {afterDisplayUrl && (
        <img src={afterDisplayUrl} alt="" aria-hidden style={{ position: 'fixed', top: '-9999px', left: '-9999px', width: 1, height: 1 }} />
      )}
      {warpedUrl && (
        <img src={warpedUrl} alt="" aria-hidden style={{ position: 'fixed', top: '-9999px', left: '-9999px', width: 1, height: 1 }} />
      )}
    </ThemeProvider>
  );
}

export default App;
