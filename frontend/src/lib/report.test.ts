import { expect, test } from 'bun:test';
import { buildReportHtml, escapeHtml } from './report';
import { DEFAULT_SETTINGS, strengthToThreshold, toForm } from './settings';
import type { AnalyzeResponse } from '../types/api';

const analysis: AnalyzeResponse = {
  stats: { pct: 1.5, changed_px: 1234, regions: 1 },
  regions: [{ rank: 1, x: 0.1, y: 0.2, w: 0.3, h: 0.1, area_px: 1234, area_pct: 1.5, score: 1, mean_heat: 88 }],
  images: {}, before_dims: { w: 1, h: 1 }, after_dims: { w: 1, h: 1 }, analysis_dims: { w: 1, h: 1 },
  resized: false, threshold: 25, baselines: 1,
  registration: { mode: 'auto', applied: true, inliers: 20, matches: 30, confidence: 0.8, shift_x: 0, shift_y: 0, message: 'aligned' },
};

test('escapeHtml neutralises markup', () => {
  expect(escapeHtml('<img src=x onerror="a">')).toBe('&lt;img src=x onerror=&quot;a&quot;&gt;');
});

test('report escapes file names and includes findings', () => {
  const html = buildReportHtml({
    generatedAt: new Date('2026-01-01T00:00:00Z'),
    before: { name: '<b>before.jpg', sha256: 'aa', dataUrl: 'data:image/jpeg;base64,AA' },
    after: { name: 'after.jpg', sha256: 'bb', dataUrl: 'data:image/jpeg;base64,BB' },
    highlightDataUrl: 'data:image/jpeg;base64,CC',
    analysis, settings: DEFAULT_SETTINGS, crops: { 1: 'data:image/jpeg;base64,DD' },
  });
  expect(html).toContain('&lt;b&gt;before.jpg');
  expect(html).not.toContain('<b>before.jpg');
  expect(html).toContain('1.50%');
  expect(html).toContain('Region 1 crop');
});

test('toForm maps strength to threshold and omits empty ignore', () => {
  expect(strengthToThreshold(75)).toBe(25);
  const fd = toForm(DEFAULT_SETTINGS, true);
  expect(fd.get('strength')).toBe('25');
  expect(fd.get('ignore')).toBeNull();
  expect(fd.get('highlight_r')).toBe('255');
  const withIgnore = toForm({ ...DEFAULT_SETTINGS, ignore: [[0, 0, 0.5, 0.5]] });
  expect(withIgnore.get('ignore')).toBe('[[0,0,0.5,0.5]]');
});

import { batchThreshold, isAnomalous } from './batch';
import type { BatchImageResult, BatchResults } from '../types/api';

test('batch threshold uses the golden spread and the absolute floor', () => {
  const r = { score_median: 1, score_spread: 0.5, min_score: 6 } as BatchResults;
  expect(batchThreshold(r, 6)).toBe(6); // 1 + 3 < floor
  expect(batchThreshold(r, 20)).toBe(11);
  const im = { score: 8, regions: [] } as unknown as BatchImageResult;
  expect(isAnomalous(im, 6)).toBe(false); // must be localizable
  expect(isAnomalous({ ...im, regions: [{ rank: 1, x: 0, y: 0, w: 1, h: 1, area_pct: 1, score: 1 }] }, 6)).toBe(true);
});
