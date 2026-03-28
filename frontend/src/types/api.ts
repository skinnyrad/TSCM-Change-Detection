export type Method = 'basic' | 'subtraction' | 'threshold' | 'heatmap' | 'advanced';

export interface AnchorPoint {
  x: number; // relative [0,1] within image display bounds
  y: number;
}

export interface PointPair {
  id: number;
  src: AnchorPoint | null; // point in "before" image
  dst: AnchorPoint | null; // point in "after" image
}

export interface AnalyzeStats {
  pct: number;
  changed_px: number;
  regions: number;
}

export interface AnalyzeImages {
  diff_map?: string;
  threshold_mask?: string;
  highlight?: string;
  subtraction?: string;
  heatmap?: string;
  edges?: string;
  contours?: string;
}

export interface Dims {
  w: number;
  h: number;
}

export interface AnalyzeResponse {
  stats: AnalyzeStats;
  images: AnalyzeImages;
  before_dims: Dims;
  after_dims: Dims;
  resized: boolean;
}
