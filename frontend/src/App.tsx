import { useState } from 'react';
import AppBar from '@mui/material/AppBar';
import Box from '@mui/material/Box';
import Container from '@mui/material/Container';
import Paper from '@mui/material/Paper';
import Tab from '@mui/material/Tab';
import Tabs from '@mui/material/Tabs';
import Toolbar from '@mui/material/Toolbar';
import Typography from '@mui/material/Typography';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import CompareRoundedIcon from '@mui/icons-material/CompareRounded';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import FlipRoundedIcon from '@mui/icons-material/FlipRounded';
import BiotechRoundedIcon from '@mui/icons-material/BiotechRounded';
import { UploadPanel } from './components/UploadPanel';
import { ImageComparisonTab } from './components/ImageComparisonTab';
import { ChangeDetectionTab } from './components/ChangeDetectionTab';
import { AdvancedAnalysisTab } from './components/AdvancedAnalysisTab';

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
  const [activeTab, setActiveTab] = useState(0);

  const bothLoaded = before !== null && after !== null;

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

      <Container maxWidth={false} sx={{ py: 3, maxWidth: '80%', mx: 'auto' }}>
        <UploadPanel
          before={before}
          after={after}
          onBefore={setBefore}
          onAfter={setAfter}
        />

        {bothLoaded && (
          <Paper variant="outlined" sx={{ mt: 2 }}>
            <Tabs
              value={activeTab}
              onChange={(_, v) => setActiveTab(v)}
              sx={{ borderBottom: '1px solid', borderColor: 'divider', px: 2 }}
            >
              <Tab icon={<CompareRoundedIcon />} iconPosition="start" label="Image Comparison" />
              <Tab icon={<SearchRoundedIcon />} iconPosition="start" label="Change Detection" />
              <Tab icon={<BiotechRoundedIcon />} iconPosition="start" label="Advanced Analysis" />
            </Tabs>

            <Box sx={{ p: 3 }}>
              {activeTab === 0 && <ImageComparisonTab before={before} after={after} />}
              {activeTab === 1 && <ChangeDetectionTab before={before} after={after} />}
              {activeTab === 2 && <AdvancedAnalysisTab before={before} after={after} />}
            </Box>
          </Paper>
        )}
      </Container>
    </ThemeProvider>
  );
}

export default App;
