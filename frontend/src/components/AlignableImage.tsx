import { useState } from 'react';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';

export const PAIR_COLORS = ['#f44336', '#4caf50', '#2196f3', '#ff9800', '#9c27b0', '#00bcd4', '#cddc39', '#ff5722'];

interface PointInfo {
  id: number;
  coords: { x: number; y: number } | null;
}

interface AlignableImageProps {
  imageUrl: string;
  side: 'src' | 'dst';
  label: string;
  points: PointInfo[];
  isActive: boolean;
  onPoint: (relX: number, relY: number) => void;
  compact?: boolean;
}

const ZOOM = 4;
const LOUPE_SIZE = 160;

interface LoupeState {
  clientX: number;
  clientY: number;
  imgX: number;
  imgY: number;
  imgW: number;
  imgH: number;
}

export function AlignableImage({ imageUrl, side, label, points, isActive, onPoint, compact }: AlignableImageProps) {
  const [loupe, setLoupe] = useState<LoupeState | null>(null);

  const handleClick = (e: React.MouseEvent<HTMLImageElement>) => {
    if (!isActive) return;
    const r = e.currentTarget.getBoundingClientRect();
    onPoint((e.clientX - r.left) / r.width, (e.clientY - r.top) / r.height);
  };

  const handleMouseMove = (e: React.MouseEvent<HTMLImageElement>) => {
    if (!isActive || compact) return;
    const r = e.currentTarget.getBoundingClientRect();
    setLoupe({
      clientX: e.clientX,
      clientY: e.clientY,
      imgX: e.clientX - r.left,
      imgY: e.clientY - r.top,
      imgW: r.width,
      imgH: r.height,
    });
  };

  const handleMouseLeave = () => setLoupe(null);

  const markerSize = compact ? 16 : 26;
  const fontSize = compact ? 8 : 12;

  // Loupe position: to the left of the cursor, vertically centered on it
  const loupeLeft = loupe ? loupe.clientX - LOUPE_SIZE - 18 : 0;
  const loupeTop = loupe ? loupe.clientY - LOUPE_SIZE / 2 : 0;

  // Background-position offsets so the cursor point is centered in the loupe
  const bgX = loupe ? -(loupe.imgX * ZOOM - LOUPE_SIZE / 2) : 0;
  const bgY = loupe ? -(loupe.imgY * ZOOM - LOUPE_SIZE / 2) : 0;

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: compact ? 0.5 : 1, alignItems: compact ? 'stretch' : 'center' }}>
      {compact && (
        <Typography variant="caption" fontWeight={600} color="text.secondary" textAlign="center" display="block">
          {label}
        </Typography>
      )}

      {/* inline-block shrink-wraps to the exact rendered image size so % markers are always correct */}
      <Box sx={{ position: 'relative', display: 'inline-block', lineHeight: 0 }}>
        <img
          src={imageUrl}
          alt={label}
          onClick={handleClick}
          onMouseMove={handleMouseMove}
          onMouseLeave={handleMouseLeave}
          draggable={false}
          style={{
            display: 'block',
            // compact: fill sidebar width; large: fit within viewport leaving room for sidebar + chrome
            ...(compact
              ? { width: '100%', height: 'auto' }
              : {
                  width: 'auto',
                  height: 'auto',
                  maxWidth: 'calc(96vw - 320px)',
                  maxHeight: 'calc(97vh - 80px)',
                }),
            cursor: isActive ? 'crosshair' : 'default',
            userSelect: 'none',
            borderRadius: compact ? 4 : 6,
          }}
        />

        {points.map(p => {
          if (!p.coords) return null;
          const color = PAIR_COLORS[(p.id - 1) % PAIR_COLORS.length];
          return (
            <Box
              key={`${side}-${p.id}`}
              sx={{
                position: 'absolute',
                left: `${p.coords.x * 100}%`,
                top: `${p.coords.y * 100}%`,
                transform: 'translate(-50%, -50%)',
                width: markerSize,
                height: markerSize,
                borderRadius: '50%',
                bgcolor: color,
                border: `${compact ? 1.5 : 2}px solid white`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                pointerEvents: 'none',
                boxShadow: '0 1px 5px rgba(0,0,0,0.7)',
              }}
            >
              <Typography variant="caption" sx={{ color: 'white', fontWeight: 700, lineHeight: 1, fontSize }}>
                {p.id}
              </Typography>
            </Box>
          );
        })}
      </Box>

      {/* Loupe — plain div with inline styles to avoid MUI mangling the blob URL */}
      {loupe && isActive && !compact && (
        <div style={{
          position: 'fixed',
          left: loupeLeft,
          top: loupeTop,
          width: LOUPE_SIZE,
          height: LOUPE_SIZE,
          borderRadius: '50%',
          border: '2px solid rgba(255,255,255,0.75)',
          boxShadow: '0 4px 20px rgba(0,0,0,0.6)',
          pointerEvents: 'none',
          zIndex: 9999,
          overflow: 'hidden',
          backgroundImage: `url(${imageUrl})`,
          backgroundSize: `${loupe.imgW * ZOOM}px ${loupe.imgH * ZOOM}px`,
          backgroundPosition: `${bgX}px ${bgY}px`,
          backgroundRepeat: 'no-repeat',
          backgroundColor: '#111',
        }}>
          {/* Crosshair */}
          <div style={{ position: 'absolute', inset: 0, pointerEvents: 'none' }}>
            <div style={{ position: 'absolute', left: '50%', top: 0, bottom: 0, width: 1, backgroundColor: 'rgba(255,80,80,0.9)', transform: 'translateX(-50%)' }} />
            <div style={{ position: 'absolute', top: '50%', left: 0, right: 0, height: 1, backgroundColor: 'rgba(255,80,80,0.9)', transform: 'translateY(-50%)' }} />
          </div>
        </div>
      )}
    </Box>
  );
}
