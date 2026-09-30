import { expect, test } from 'bun:test';
import { IDENTITY, MAX_ZOOM, clampView, panBy, visibleRect, zoomAt } from './zoom';

const W = 800, H = 600;

test('zooming keeps the point under the cursor fixed', () => {
  const v = zoomAt(IDENTITY, 2, 200, 150, W, H);
  expect(v.s).toBe(2);
  // Content point under the cursor: (px - x) / s must be unchanged (200,150).
  expect((200 - v.x) / v.s).toBeCloseTo(200);
  expect((150 - v.y) / v.s).toBeCloseTo(150);
});

test('scale is clamped and zooming out fully returns to identity', () => {
  expect(zoomAt(IDENTITY, 0.5, 100, 100, W, H)).toEqual(IDENTITY);
  expect(zoomAt(IDENTITY, 1000, 100, 100, W, H).s).toBe(MAX_ZOOM);
  const zoomed = zoomAt(IDENTITY, 3, 700, 500, W, H);
  const back = zoomAt(zoomed, 1 / 3, 10, 10, W, H);
  expect(back.s).toBe(1);
  expect(back.x).toBeCloseTo(0);
  expect(back.y).toBeCloseTo(0);
});

test('panning cannot reveal empty space', () => {
  const v = zoomAt(IDENTITY, 2, 0, 0, W, H); // showing the top-left quarter
  expect(panBy(v, 50, 50, W, H)).toEqual({ s: 2, x: 0, y: 0 });
  const far = panBy(v, -10000, -10000, W, H);
  expect(far.x).toBe(W - W * 2);
  expect(far.y).toBe(H - H * 2);
  expect(clampView({ s: 1, x: 30, y: -30 }, W, H)).toEqual({ s: 1, x: 0, y: 0 });
});

test('visible rect is the on-screen part of the content, as fractions', () => {
  expect(visibleRect(IDENTITY, W, H)).toEqual({ x: 0, y: 0, w: 1, h: 1 });
  // Zoomed 2× and panned fully to the bottom-right: the bottom-right quarter is visible.
  const v = panBy(zoomAt(IDENTITY, 2, 0, 0, W, H), -10000, -10000, W, H);
  const r = visibleRect(v, W, H);
  expect(r.x).toBeCloseTo(0.5);
  expect(r.y).toBeCloseTo(0.5);
  expect(r.w).toBeCloseTo(0.5);
});
