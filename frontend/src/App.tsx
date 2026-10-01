import { useCallback, useEffect, useState } from 'react';
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
import { BatchPage } from './components/BatchPage';
import ToggleButton from '@mui/material/ToggleButton';
import ToggleButtonGroup from '@mui/material/ToggleButtonGroup';
import { getJson, postForm } from './lib/api';
import { DEFAULT_SETTINGS, type DetectionSettings } from './lib/settings';

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

const tabProps = (i: number) => ({ id: `tab-${i}`, 'aria-controls': `tabpanel-${i}` });

export function App() {
  const [before, setBefore] = useState<File | null>(null);
  const [after, setAfter] = useState<File | null>(null);
  const [warpedUrl, setWarpedUrl] = useState<string | null>(null);
  const [alignDialogOpen, setAlignDialogOpen] = useState(false);
  const [activeTab, setActiveTab] = useState(0);
  const [mode, setMode] = useState<'pair' | 'batch'>('pair');
  const [settings, setSettings] = useState<DetectionSettings>(DEFAULT_SETTINGS);
  // Auto-align lives on the server; mirror it here (not in the tab, which
  // unmounts on tab switches) and read it once in case it was changed earlier.
  const [autoAlign, setAutoAlign] = useState(true);
  useEffect(() => {
    getJson<{ auto: boolean }>('/api/registration').then(r => setAutoAlign(r.auto)).catch(() => {});
  }, []);
  const patchSettings = useCallback((p: Partial<DetectionSettings>) => setSettings(s => ({ ...s, ...p })), []);

  const {
    uploadBefore, uploadAfter, uploadBaseline, clearBaselines, refresh,
    uploadingBefore, uploadingAfter, uploadingBaseline, error: uploadError,
    beforeDims, afterDims, beforeDisplayUrl, afterDisplayUrl, baselines, ready, imageKey,
  } = useUpload();

  // Uploading a new image invalidates any manual warp on the server as well,
  // so only the client-side preview URL needs dropping here.
  const dropWarpPreview = () => {
    if (warpedUrl) URL.revokeObjectURL(warpedUrl);
    setWarpedUrl(null);
  };

  const handleBefore = (f: File) => {
    dropWarpPreview();
    setBefore(f);
    uploadBefore(f);
  };

  const handleAfter = (f: File) => {
    dropWarpPreview();
    setAfter(f);
    uploadAfter(f);
  };

  const handleAligned = (url: string) => {
    if (warpedUrl) URL.revokeObjectURL(warpedUrl);
    const img = new Image();
    img.src = url;
    img.decode().catch(() => {});
    setWarpedUrl(url);
    setActiveTab(0);
    refresh();
  };

  const resetAlignment = async () => {
    try {
      await postForm('/api/clear-warp');
    } finally {
      dropWarpPreview();
      refresh();
    }
  };

  const bothSelected = before !== null && after !== null;
  const comparisonBeforeUrl = warpedUrl ?? beforeDisplayUrl ?? '';

  return (
    <ThemeProvider theme={darkTheme}>
      <CssBaseline />
      <AppBar position="static" elevation={0} sx={{ borderBottom: '1px solid', borderColor: 'divider' }}>
        <Toolbar>
          <FlipRoundedIcon sx={{ mr: 1.5 }} />
          <Typography variant="h6" component="h1" fontWeight={700}>
            TSCM Change Detection
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ ml: 2, display: { xs: 'none', md: 'block' } }}>
            {mode === 'pair'
              ? 'Upload a Before and After image to identify changes between them.'
              : 'Upload many photos of one scene to find the ones that differ.'}
          </Typography>
          <ToggleButtonGroup
            value={mode} exclusive size="small" aria-label="Mode" sx={{ ml: 'auto' }}
            onChange={(_, v) => { if (v) setMode(v); }}
          >
            <ToggleButton value="pair">Compare two</ToggleButton>
            <ToggleButton value="batch">Batch anomalies</ToggleButton>
          </ToggleButtonGroup>
        </Toolbar>
      </AppBar>

      {(uploadingBefore || uploadingAfter || uploadingBaseline) && <LinearProgress aria-label="Uploading" />}

      {mode === 'batch' && (
        <Container maxWidth={false} sx={{ py: 3, maxWidth: '80%', mx: 'auto' }}>
          <BatchPage />
        </Container>
      )}

      <Container maxWidth={false} sx={{ py: 3, maxWidth: '80%', mx: 'auto', display: mode === 'pair' ? undefined : 'none' }}>
        <UploadPanel
          before={before}
          after={after}
          onBefore={handleBefore}
          onAfter={handleAfter}
          onBaseline={uploadBaseline}
          onClearBaselines={clearBaselines}
          onResetAlignment={resetAlignment}
          beforeDisplayUrl={warpedUrl ?? beforeDisplayUrl}
          afterDisplayUrl={afterDisplayUrl}
          baselines={baselines}
          uploadingBaseline={uploadingBaseline}
          alignmentActive={warpedUrl !== null}
          error={uploadError}
        />

        {bothSelected && (
          <Paper variant="outlined">
            <Tabs
              value={activeTab}
              onChange={(_, v) => setActiveTab(v)}
              aria-label="Analysis views"
              sx={{ borderBottom: '1px solid', borderColor: 'divider', px: 2 }}
            >
              <Tab icon={<CompareRoundedIcon />} iconPosition="start" label="Image Comparison" {...tabProps(0)} />
              <Tab icon={<SearchRoundedIcon />} iconPosition="start" label="Change Detection" {...tabProps(1)} />
              <Tab icon={<BiotechRoundedIcon />} iconPosition="start" label="Alternate Analysis" {...tabProps(2)} />
            </Tabs>

            <Box sx={{ p: 3 }} role="tabpanel" id={`tabpanel-${activeTab}`} aria-labelledby={`tab-${activeTab}`}>
              {activeTab === 0 && beforeDisplayUrl && afterDisplayUrl && (
                <ImageComparisonTab beforeUrl={comparisonBeforeUrl} afterUrl={afterDisplayUrl} />
              )}
              {activeTab === 1 && (
                <ChangeDetectionTab
                  ready={ready}
                  imageKey={imageKey}
                  settings={settings}
                  onSettings={patchSettings}
                  beforeUrl={comparisonBeforeUrl}
                  afterUrl={afterDisplayUrl ?? ''}
                  beforeFile={before}
                  afterFile={after}
                  onRefresh={refresh}
                  autoAlign={autoAlign}
                  onAutoAlign={setAutoAlign}
                />
              )}
              {activeTab === 2 && <AlternateAnalysisTab ready={ready} imageKey={imageKey} settings={settings} />}
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

      {ready && mode === 'pair' && (
        <Tooltip title={warpedUrl ? 'Edit manual alignment' : 'Align images manually'} placement="left">
          <Fab
            aria-label={warpedUrl ? 'Edit manual alignment' : 'Align images manually'}
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
    </ThemeProvider>
  );
}

export default App;
