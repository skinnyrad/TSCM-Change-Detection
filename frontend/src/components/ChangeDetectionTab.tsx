import { useEffect, useState } from "react";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Divider from "@mui/material/Divider";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import Slider from "@mui/material/Slider";
import Typography from "@mui/material/Typography";
import { useAnalyze } from "../hooks/useAnalyze";
import type { Method } from "../types/api";
import { ResultImage } from "./ResultImage";
import { StatsBar } from "./StatsBar";

interface ChangeDetectionTabProps {
  ready: boolean;
  imageKey: string;
}

const METHOD_LABELS: Record<Method, string> = {
  basic: "Basic Difference",
  subtraction: "Image Subtraction",
  threshold: "Threshold Detection",
  heatmap: "Heat Map",
  advanced: "Advanced Analysis",
};

export function ChangeDetectionTab({
  ready,
  imageKey,
}: ChangeDetectionTabProps) {
  const [method, setMethod] = useState<Method>("basic");
  const [strength, setStrength] = useState(65);
  const [morphSize, setMorphSize] = useState(3);
  const [minRegion, setMinRegion] = useState(50);

  const { data, error, analyze } = useAnalyze({
    method,
    strength,
    minRegion,
    morphSize,
    ready,
  });

  // Auto-analyze when params change, debounced so StrictMode's unmount/remount
  // cycle cancels the timer rather than aborting an HTTP request.
  useEffect(() => {
    const timer = setTimeout(analyze, 300);
    return () => clearTimeout(timer);
  }, [ready, imageKey, method, strength, morphSize, minRegion, analyze]);

  const images = data?.images;

  return (
    <Box>
      {/* Controls */}
      <Box
        sx={{
          display: "flex",
          gap: 2,
          alignItems: "flex-end",
          mb: 3,
          flexWrap: "wrap",
        }}
      >
        <FormControl size="small" sx={{ minWidth: 200 }}>
          <InputLabel>Detection Method</InputLabel>
          <Select
            value={method}
            label="Detection Method"
            onChange={(e) => setMethod(e.target.value as Method)}
          >
            {(["basic", "subtraction", "threshold", "heatmap"] as Method[]).map(
              (m) => (
                <MenuItem key={m} value={m}>
                  {METHOD_LABELS[m]}
                </MenuItem>
              ),
            )}
          </Select>
        </FormControl>

        {method !== "subtraction" && (
          <Box sx={{ minWidth: 240 }}>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              Detection Strength: {strength}
            </Typography>
            <Slider
              value={strength}
              min={5}
              max={100}
              step={1}
              onChange={(_, v) => setStrength(v as number)}
              size="small"
            />
          </Box>
        )}

        <Box sx={{ minWidth: 200 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Noise Reduction:{" "}
            {morphSize <= 1 ? "off" : `${morphSize}×${morphSize}`}
          </Typography>
          <Slider
            value={morphSize}
            min={1}
            max={11}
            step={1}
            marks
            onChange={(_, v) => setMorphSize(v as number)}
            size="small"
          />
        </Box>

        <Box sx={{ minWidth: 200 }}>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Min Region Size: {minRegion} px
          </Typography>
          <Slider
            value={minRegion}
            min={1}
            max={500}
            step={1}
            onChange={(_, v) => setMinRegion(v as number)}
            size="small"
          />
        </Box>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {data && (
        <>
          <StatsBar stats={data.stats} />
          <Divider sx={{ mb: 3 }} />

          <Box
            sx={{
              display: "grid",
              gap: 2,
              gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))",
            }}
          >
            {method === "basic" && (
              <>
                {images?.diff_map && (
                  <ResultImage src={images.diff_map} caption="Difference Map" />
                )}
                {images?.threshold_mask && (
                  <ResultImage
                    src={images.threshold_mask}
                    caption="Thresholded Changes"
                  />
                )}
                {images?.highlight && (
                  <ResultImage
                    src={images.highlight}
                    caption="Changes Highlighted on After"
                  />
                )}
              </>
            )}
            {method === "subtraction" && (
              <>
                {images?.subtraction && (
                  <ResultImage
                    src={images.subtraction}
                    caption="Subtraction Result"
                  />
                )}
                {images?.highlight && (
                  <ResultImage
                    src={images.highlight}
                    caption="Changes Highlighted on After"
                  />
                )}
              </>
            )}
            {method === "threshold" && (
              <>
                {images?.threshold_mask && (
                  <ResultImage
                    src={images.threshold_mask}
                    caption={`Threshold Result (strength=${strength})`}
                  />
                )}
                {images?.highlight && (
                  <ResultImage
                    src={images.highlight}
                    caption="Changes Highlighted on After"
                  />
                )}
              </>
            )}
            {method === "heatmap" && (
              <>
                {images?.heatmap && (
                  <ResultImage
                    src={images.heatmap}
                    caption="Change Intensity Heat Map"
                  />
                )}
                {images?.highlight && (
                  <ResultImage
                    src={images.highlight}
                    caption="Changes Highlighted on After"
                  />
                )}
              </>
            )}
          </Box>
        </>
      )}
    </Box>
  );
}
