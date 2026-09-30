import type { AnalyzeResponse, Region } from '../types/api';
import type { DetectionSettings } from './settings';

export interface ReportInput {
  generatedAt: Date;
  before: { name: string; sha256: string; dataUrl: string };
  after: { name: string; sha256: string; dataUrl: string };
  highlightDataUrl: string;
  analysis: AnalyzeResponse;
  settings: DetectionSettings;
  /** Crops keyed by region rank (After image). */
  crops: Record<number, string>;
}

export const escapeHtml = (s: string) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');

const regionRow = (r: Region, crop?: string) => `
<tr>
  <td>${r.rank}</td>
  <td>${crop ? `<img class="crop" src="${crop}" alt="Region ${r.rank} crop">` : ''}</td>
  <td>${r.area_pct.toFixed(2)}% (${r.area_px.toLocaleString('en-US')} px)</td>
  <td>${Math.round(r.score * 100)}%</td>
  <td>${r.mean_heat.toFixed(0)}</td>
  <td>x ${(r.x * 100).toFixed(0)}–${((r.x + r.w) * 100).toFixed(0)}%, y ${(r.y * 100).toFixed(0)}–${((r.y + r.h) * 100).toFixed(0)}%</td>
</tr>`;

/** A single self-contained HTML document (images embedded) that prints cleanly to PDF. */
export function buildReportHtml(i: ReportInput): string {
  const a = i.analysis;
  const s = i.settings;
  const reg = a.registration;
  const rows = a.regions.map(r => regionRow(r, i.crops[r.rank])).join('');
  const settings: [string, string][] = [
    ['Detection strength', `${s.strength}${s.adaptiveThreshold ? ' (adaptive threshold)' : ''}`],
    ['Threshold used', String(a.threshold)],
    ['Noise reduction', `${s.morphSize}×${s.morphSize}`],
    ['Fill gaps', `${s.closeSize}×${s.closeSize}`],
    ['Min region size', `${s.minRegion} px`],
    ['Pre-blur σ', String(s.preBlurSigma)],
    ['Exposure matching', s.matchIntensity ? 'on' : 'off'],
    ['Colour-aware', s.colorAware ? 'on' : 'off'],
    ['Shift tolerance', `±${s.shiftTolerance} px`],
    ['Baselines combined', String(a.baselines)],
    ['Ignore zones', String(s.ignore.length)],
    ['Alignment', `${reg.mode}${reg.message ? ` — ${reg.message}` : ''}`],
  ];
  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>TSCM Change Detection Report</title>
<style>
  body{font:14px/1.5 system-ui,sans-serif;margin:2rem auto;max-width:1000px;padding:0 1rem;color:#111}
  h1{margin-bottom:0} h2{margin-top:2rem;border-bottom:1px solid #ccc;padding-bottom:.2rem}
  table{border-collapse:collapse;width:100%} td,th{border:1px solid #ccc;padding:.35rem .5rem;text-align:left;vertical-align:top}
  img{max-width:100%} .crop{max-height:110px} .grid{display:grid;grid-template-columns:1fr 1fr;gap:1rem}
  code{font-size:11px;word-break:break-all} .muted{color:#555}
  @media print{body{margin:0} h2{break-after:avoid} tr,img{break-inside:avoid}}
</style></head><body>
<h1>TSCM Change Detection Report</h1>
<p class="muted">Generated ${escapeHtml(i.generatedAt.toISOString())}</p>

<h2>Summary</h2>
<p><strong>${a.stats.regions}</strong> distinct region${a.stats.regions === 1 ? '' : 's'} detected; <strong>${a.stats.pct}%</strong> of the image changed (${a.stats.changed_px.toLocaleString('en-US')} px).
${a.regions.length < a.stats.regions ? `The top ${a.regions.length} are listed.` : ''}</p>

<h2>Source images</h2>
<table>
<tr><th>Role</th><th>File</th><th>SHA-256</th></tr>
<tr><td>Before</td><td>${escapeHtml(i.before.name)}</td><td><code>${escapeHtml(i.before.sha256)}</code></td></tr>
<tr><td>After</td><td>${escapeHtml(i.after.name)}</td><td><code>${escapeHtml(i.after.sha256)}</code></td></tr>
</table>

<h2>Images</h2>
<div class="grid"><figure><img src="${i.before.dataUrl}" alt="Before"><figcaption>Before</figcaption></figure>
<figure><img src="${i.after.dataUrl}" alt="After"><figcaption>After</figcaption></figure></div>
<figure><img src="${i.highlightDataUrl}" alt="Changes highlighted"><figcaption>Changes highlighted on After</figcaption></figure>

<h2>Findings</h2>
${a.regions.length === 0 ? '<p>No changes detected at these settings.</p>' : `<table>
<tr><th>#</th><th>Crop (After)</th><th>Area</th><th>Relative strength</th><th>Mean change</th><th>Location</th></tr>${rows}</table>`}

<h2>Settings</h2>
<table>${settings.map(([k, v]) => `<tr><th>${escapeHtml(k)}</th><td>${escapeHtml(v)}</td></tr>`).join('')}</table>
</body></html>`;
}
