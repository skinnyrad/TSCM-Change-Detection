import type { BatchImageResult, BatchRegion, BatchResults, Region } from '../types/api';
import { escapeHtml } from './report';

/**
 * Anomaly threshold for a strictness k: an image must stand k robust
 * deviations above the golden images and clear the absolute score floor
 * (it must also contain at least one region — see isAnomalous).
 * Mirrors imgproc.BatchResult.Threshold so the slider can re-classify live.
 */
export function batchThreshold(r: BatchResults, k: number): number {
  return Math.max(r.score_median + k * r.score_spread, r.min_score);
}

export const isAnomalous = (im: BatchImageResult, threshold: number) => im.score >= threshold && im.regions.length > 0;

/** Adapt batch regions to the shape ImageOverlay draws. */
export const toOverlayRegions = (regions: BatchRegion[]): Region[] =>
  regions.map(r => ({ ...r, area_px: 0, mean_heat: 0 }));

export const alignedUrl = (id: string, size = 1024) => `/api/batch/image/${id}?kind=aligned&size=${size}`;
export const heatUrl = (id: string) => `/api/batch/image/${id}?kind=heat`;
export const REFERENCE_URL = '/api/batch/reference';

export interface BatchReportEntry {
  image: BatchImageResult;
  /** Aligned image with heat and boxes composited, as a data URL. */
  preview: string;
}

/** Self-contained HTML report for a batch analysis (prints cleanly to PDF). */
export function buildBatchReportHtml(opts: {
  generatedAt: Date;
  results: BatchResults;
  sensitivity: number;
  threshold: number;
  referenceDataUrl: string;
  anomalies: BatchReportEntry[];
}): string {
  const { results: r } = opts;
  const rows = opts.anomalies.map(({ image: im, preview }, i) => `
<section class="finding">
  <h3>${i + 1}. ${escapeHtml(im.name)}</h3>
  <p>Anomaly score ${im.score.toFixed(1)} (robust z ${im.z.toFixed(1)}) · ${im.regions.length} region${im.regions.length === 1 ? '' : 's'}${im.registered ? '' : ' · <strong>could not be aligned to the scene</strong>'}</p>
  <img src="${preview}" alt="${escapeHtml(im.name)} with anomalies highlighted">
  ${im.regions.length ? `<table><tr><th>#</th><th>Location</th><th>Area</th><th>Relative strength</th></tr>${im.regions.map(g => `
    <tr><td>${g.rank}</td><td>x ${(g.x * 100).toFixed(0)}–${((g.x + g.w) * 100).toFixed(0)}%, y ${(g.y * 100).toFixed(0)}–${((g.y + g.h) * 100).toFixed(0)}%</td><td>${g.area_pct.toFixed(2)}%</td><td>${Math.round(g.score * 100)}%</td></tr>`).join('')}</table>` : ''}
</section>`).join('');
  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>TSCM Batch Anomaly Report</title>
<style>
  body{font:14px/1.5 system-ui,sans-serif;margin:2rem auto;max-width:1000px;padding:0 1rem;color:#111}
  h1{margin-bottom:0} h2{margin-top:2rem;border-bottom:1px solid #ccc;padding-bottom:.2rem}
  table{border-collapse:collapse;width:100%;margin-top:.5rem} td,th{border:1px solid #ccc;padding:.3rem .5rem;text-align:left}
  img{max-width:100%} .muted{color:#555} .finding{break-inside:avoid;margin-bottom:2rem}
  .tiles{display:grid;grid-template-columns:repeat(4,1fr);gap:.5rem} .tile{border:1px solid #ccc;border-radius:6px;padding:.5rem;text-align:center}
  .tile b{display:block;font-size:1.4rem}
</style></head><body>
<h1>TSCM Batch Anomaly Report</h1>
<p class="muted">Generated ${escapeHtml(opts.generatedAt.toISOString())}</p>
<div class="tiles">
  <div class="tile"><b>${r.images.length}</b>images</div>
  <div class="tile"><b>${opts.anomalies.length}</b>anomalous</div>
  <div class="tile"><b>${r.golden_count}</b>in golden set</div>
  <div class="tile"><b>${r.unregistered}</b>not aligned</div>
</div>
<h2>Method</h2>
<p>All images were aligned to a reference shot and exposure-matched. A per-pixel median and spread were computed over the
"golden" set (images that agree with each other), trimming outliers iteratively. Each image was scored by its strongest
blob of deviation from that golden set. Images scoring at least ${opts.threshold.toFixed(1)} (sensitivity ${opts.sensitivity}) are listed below.</p>
<figure><img src="${opts.referenceDataUrl}" alt="Golden reference"><figcaption>Golden reference (per-pixel median of the golden set)</figcaption></figure>
<h2>Anomalous images</h2>
${rows || '<p>No anomalies at this sensitivity.</p>'}
</body></html>`;
}
