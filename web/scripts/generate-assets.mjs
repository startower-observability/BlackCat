/**
 * Placeholder pixel art sprite generator for Pixel Cat Dashboard.
 * Generates cat-spritesheet.png, room-bg.png, desk sprites, and effect sprites
 * using pure Node.js Buffer PNG writing (no native deps).
 *
 * Run: node scripts/generate-assets.mjs
 */

import fs from 'fs';
import path from 'path';
import { createRequire } from 'module';
import zlib from 'zlib';

// ─── Pure-JS PNG Writer ───────────────────────────────────────────────────────

function crc32(buf) {
  let crc = 0xffffffff;
  const table = new Uint32Array(256);
  for (let i = 0; i < 256; i++) {
    let c = i;
    for (let k = 0; k < 8; k++) c = (c & 1) ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    table[i] = c;
  }
  for (let i = 0; i < buf.length; i++) crc = table[(crc ^ buf[i]) & 0xff] ^ (crc >>> 8);
  return (crc ^ 0xffffffff) >>> 0;
}

function u32be(n) {
  const b = Buffer.alloc(4);
  b.writeUInt32BE(n, 0);
  return b;
}

function chunk(type, data) {
  const typeBytes = Buffer.from(type, 'ascii');
  const crcInput = Buffer.concat([typeBytes, data]);
  return Buffer.concat([u32be(data.length), typeBytes, data, u32be(crc32(crcInput))]);
}

function writePNG(w, h, pixels) {
  // pixels: Uint8Array of RGBA, row-major
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);

  // IHDR
  const ihdrData = Buffer.alloc(13);
  ihdrData.writeUInt32BE(w, 0);
  ihdrData.writeUInt32BE(h, 4);
  ihdrData[8] = 8;  // bit depth
  ihdrData[9] = 6;  // RGBA color type
  ihdrData[10] = 0; ihdrData[11] = 0; ihdrData[12] = 0;

  // IDAT - build raw scanlines then deflate
  const rawSize = h * (1 + w * 4);
  const raw = Buffer.alloc(rawSize);
  let offset = 0;
  for (let y = 0; y < h; y++) {
    raw[offset++] = 0; // filter type None
    for (let x = 0; x < w; x++) {
      const i = (y * w + x) * 4;
      raw[offset++] = pixels[i];     // R
      raw[offset++] = pixels[i + 1]; // G
      raw[offset++] = pixels[i + 2]; // B
      raw[offset++] = pixels[i + 3]; // A
    }
  }

  const compressed = zlib.deflateSync(raw, { level: 6 });

  // IEND
  return Buffer.concat([
    signature,
    chunk('IHDR', ihdrData),
    chunk('IDAT', compressed),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

// ─── Canvas Abstraction ───────────────────────────────────────────────────────

function createCanvas(w, h) {
  const pixels = new Uint8Array(w * h * 4); // RGBA, initialized to 0 (transparent)

  function hexToRGBA(hex, alpha = 255) {
    const h = hex.replace('#', '');
    return [
      parseInt(h.slice(0, 2), 16),
      parseInt(h.slice(2, 4), 16),
      parseInt(h.slice(4, 6), 16),
      alpha,
    ];
  }

  function setPixel(x, y, r, g, b, a = 255) {
    if (x < 0 || x >= w || y < 0 || y >= h) return;
    const i = (y * w + x) * 4;
    pixels[i] = r; pixels[i+1] = g; pixels[i+2] = b; pixels[i+3] = a;
  }

  function fillRect(x1, y1, rw, rh, r, g, b, a = 255) {
    for (let y = y1; y < y1 + rh; y++)
      for (let x = x1; x < x1 + rw; x++)
        setPixel(x, y, r, g, b, a);
  }

  function fillRectHex(x1, y1, rw, rh, hex, alpha = 255) {
    const [r, g, b] = hexToRGBA(hex);
    fillRect(x1, y1, rw, rh, r, g, b, alpha);
  }

  function drawLine(x1, y1, x2, y2, r, g, b, a = 255) {
    const dx = Math.abs(x2 - x1), dy = Math.abs(y2 - y1);
    const sx = x1 < x2 ? 1 : -1, sy = y1 < y2 ? 1 : -1;
    let err = dx - dy;
    while (true) {
      setPixel(x1, y1, r, g, b, a);
      if (x1 === x2 && y1 === y2) break;
      const e2 = 2 * err;
      if (e2 > -dy) { err -= dy; x1 += sx; }
      if (e2 < dx) { err += dx; y1 += sy; }
    }
  }

  // Simple 5×7 pixel font for letters
  const font5x7 = {
    'W': [[1,0,1],[1,0,1],[1,0,1],[1,1,1],[1,1,1]],
    'I': [[1],[1],[1],[1],[1]],
    'E': [[1,1],[1,0],[1,1],[1,0],[1,1]],
    'Z': [[1,1],[0,1],[1,1],[1,0],[1,1]],
    '!': [[1],[1],[1],[0],[1]],
    '?': [[1,1],[0,1],[0,1],[0,0],[0,1]],
    '*': [[0,1,0],[1,1,1],[0,1,0]],
  };

  function drawChar(char, cx, cy, scale, r, g, b, a = 255) {
    const glyph = font5x7[char];
    if (!glyph) return;
    glyph.forEach((row, dy) => {
      row.forEach((bit, dx) => {
        if (bit) {
          for (let sy = 0; sy < scale; sy++)
            for (let sx = 0; sx < scale; sx++)
              setPixel(cx + dx * scale + sx, cy + dy * scale + sy, r, g, b, a);
        }
      });
    });
  }

  return { pixels, w, h, fillRect, fillRectHex, hexToRGBA, drawLine, setPixel, drawChar };
}

// ─── Output Dirs ─────────────────────────────────────────────────────────────

const __dir = path.dirname(new URL(import.meta.url).pathname).replace(/^\/([A-Z]:)/, '$1');
const outDir = path.resolve(__dir, '../public/sprites');
const effectsDir = path.join(outDir, 'effects');

fs.mkdirSync(outDir, { recursive: true });
fs.mkdirSync(effectsDir, { recursive: true });
console.log('Output:', outDir);

// ─── Cat Spritesheet ─────────────────────────────────────────────────────────
// 5 cols × 4 rows, 48×48 each = 240×192px
// Row 0: working_0..3, success_0
// Row 1: idle_0..2, success_1, success_2
// Row 2: error_0..3, success_3
// Row 3: thinking_0..3, success_4

const FRAME = 48;
const SHEET_W = 240, SHEET_H = 192;

const stateColors = {
  working:  '#1f6feb',
  idle:     '#8b949e',
  error:    '#da3633',
  thinking: '#d29922',
  success:  '#238636',
};

const stateLetters = {
  working: 'W', idle: 'I', error: 'E', thinking: 'Z', success: '*',
};

const frameLayout = [
  // [state, frameIndex, col, row]
  ['working', 0, 0, 0], ['working', 1, 1, 0], ['working', 2, 2, 0], ['working', 3, 3, 0], ['success', 0, 4, 0],
  ['idle',    0, 0, 1], ['idle',    1, 1, 1], ['idle',    2, 2, 1], ['success', 1, 3, 1], ['success', 2, 4, 1],
  ['error',   0, 0, 2], ['error',   1, 1, 2], ['error',   2, 2, 2], ['error',   3, 3, 2], ['success', 3, 4, 2],
  ['thinking',0, 0, 3], ['thinking',1, 1, 3], ['thinking',2, 2, 3], ['thinking',3, 3, 3], ['success', 4, 4, 3],
];

// Build JSON atlas
const frames = {};
const animations = { working: [], idle: [], error: [], thinking: [], success: [] };

for (const [state, idx, col, row] of frameLayout) {
  const key = `${state}_${idx}`;
  frames[key] = {
    frame: { x: col * FRAME, y: row * FRAME, w: FRAME, h: FRAME },
    sourceSize: { w: FRAME, h: FRAME },
    spriteSourceSize: { x: 0, y: 0, w: FRAME, h: FRAME },
  };
  animations[state].push(key);
}

const atlas = {
  frames,
  animations,
  meta: {
    image: 'cat-spritesheet.png',
    size: { w: SHEET_W, h: SHEET_H },
    scale: '1',
  },
};

fs.writeFileSync(path.join(outDir, 'cat-spritesheet.json'), JSON.stringify(atlas, null, 2));
console.log('✓ cat-spritesheet.json');

// Draw spritesheet PNG
const sheet = createCanvas(SHEET_W, SHEET_H);

for (const [state, idx, col, row] of frameLayout) {
  const ox = col * FRAME, oy = row * FRAME;
  const [r, g, b] = sheet.hexToRGBA(stateColors[state]);

  // Slight shade variation per frame for animation feel
  const shade = 1 - idx * 0.08;
  const fr = Math.min(255, Math.round(r * shade));
  const fg = Math.min(255, Math.round(g * shade));
  const fb = Math.min(255, Math.round(b * shade));

  // Fill transparent first (already 0)
  // Draw body: 32×32 centered, pixel-art rounded look (just square for placeholder)
  const bx = ox + 8, by = oy + 8, bw = 32, bh = 32;
  sheet.fillRect(bx, by, bw, bh, fr, fg, fb);

  // Draw "ears" (2×2 squares at top corners)
  sheet.fillRect(bx, by - 4, 6, 4, fr, fg, fb);
  sheet.fillRect(bx + bw - 6, by - 4, 6, 4, fr, fg, fb);

  // Draw letter inside
  const letter = stateLetters[state];
  sheet.drawChar(letter, bx + 10, by + 10, 2, 255, 255, 255);
}

fs.writeFileSync(path.join(outDir, 'cat-spritesheet.png'), writePNG(SHEET_W, SHEET_H, sheet.pixels));
console.log('✓ cat-spritesheet.png');

// ─── Room Background 640×480 ─────────────────────────────────────────────────

{
  const c = createCanvas(640, 480);

  // Base floor
  c.fillRectHex(0, 0, 640, 480, '#0d1117');
  // Floor area (inner)
  c.fillRectHex(32, 96, 576, 352, '#1c1c2e');
  // Wall (top)
  c.fillRectHex(0, 0, 640, 96, '#161620');
  // Left wall
  c.fillRectHex(0, 96, 32, 352, '#161620');
  // Right wall
  c.fillRectHex(608, 96, 32, 352, '#161620');
  // Bottom strip
  c.fillRectHex(0, 448, 640, 32, '#161620');

  // Floor grid lines (32px grid, subtle)
  const [gr, gg, gb] = c.hexToRGBA('#1a1a2e');
  for (let x = 32; x < 608; x += 32) c.drawLine(x, 96, x, 448, gr, gg, gb, 180);
  for (let y = 96; y < 448; y += 32) c.drawLine(32, y, 608, y, gr, gg, gb, 180);

  // Bookshelf top-right
  c.fillRectHex(480, 16, 128, 72, '#1a1a2e');
  for (let sy = 28; sy < 80; sy += 16) c.fillRectHex(484, sy, 120, 2, '#2a2a3a');

  // Window top-left
  c.fillRectHex(48, 12, 96, 60, '#162032');
  c.fillRectHex(56, 18, 80, 48, '#1a3a5c');
  c.fillRectHex(94, 18, 4, 48, '#162032'); // window divider

  // Plant bottom-left
  c.fillRectHex(48, 392, 32, 48, '#1a3a1a');
  c.fillRectHex(56, 376, 16, 24, '#143214');

  // Rug center
  c.fillRectHex(220, 240, 200, 140, '#1a1030');
  // Rug border
  const [rr, rg, rb] = c.hexToRGBA('#2a1848');
  for (let x = 220; x < 420; x++) { c.setPixel(x, 240, rr, rg, rb); c.setPixel(x, 379, rr, rg, rb); }
  for (let y = 240; y < 380; y++) { c.setPixel(220, y, rr, rg, rb); c.setPixel(419, y, rr, rg, rb); }

  fs.writeFileSync(path.join(outDir, 'room-bg.png'), writePNG(640, 480, c.pixels));
  console.log('✓ room-bg.png');
}

// ─── Desk Back 200×80 ────────────────────────────────────────────────────────

{
  const c = createCanvas(200, 80);
  // Desk surface
  c.fillRectHex(0, 0, 200, 80, '#2d2d42');
  // Laptop
  c.fillRectHex(20, 16, 60, 40, '#1a1a2e');
  c.fillRectHex(24, 20, 52, 32, '#0d3060');
  // Coffee mug
  c.fillRectHex(140, 28, 20, 24, '#3a2020');
  c.fillRectHex(144, 32, 12, 16, '#2a1515');
  // Paper stack
  c.fillRectHex(100, 16, 28, 20, '#2a2a3a');
  c.fillRectHex(104, 12, 24, 4, '#3a3a4a');
  // Desk edge shadow
  c.fillRectHex(0, 74, 200, 6, '#222233');

  fs.writeFileSync(path.join(outDir, 'desk-back.png'), writePNG(200, 80, c.pixels));
  console.log('✓ desk-back.png');
}

// ─── Desk Front 200×20 ───────────────────────────────────────────────────────

{
  const c = createCanvas(200, 20);
  // Transparent top 4px, then desk edge
  c.fillRectHex(0, 4, 200, 2, '#2a2a3a');
  c.fillRectHex(0, 6, 200, 14, '#22222f');

  fs.writeFileSync(path.join(outDir, 'desk-front.png'), writePNG(200, 20, c.pixels));
  console.log('✓ desk-front.png');
}

// ─── Effect Sprites 32×32 ────────────────────────────────────────────────────

// zzz-bubble
{
  const c = createCanvas(32, 32);
  // 3 white ascending squares representing ZZZs
  c.fillRect(4, 20, 8, 8, 255, 255, 255);
  c.fillRect(12, 12, 8, 8, 220, 220, 220);
  c.fillRect(20, 4, 8, 8, 180, 180, 180);
  fs.writeFileSync(path.join(effectsDir, 'zzz-bubble.png'), writePNG(32, 32, c.pixels));
  console.log('✓ effects/zzz-bubble.png');
}

// alert-bubble
{
  const c = createCanvas(32, 32);
  // Two red vertical bars "!!"
  c.fillRect(8, 4, 6, 18, 218, 54, 51);
  c.fillRect(8, 26, 6, 4, 218, 54, 51);
  c.fillRect(18, 4, 6, 18, 218, 54, 51);
  c.fillRect(18, 26, 6, 4, 218, 54, 51);
  fs.writeFileSync(path.join(effectsDir, 'alert-bubble.png'), writePNG(32, 32, c.pixels));
  console.log('✓ effects/alert-bubble.png');
}

// sparkle
{
  const c = createCanvas(32, 32);
  const [yr, yg, yb] = [255, 210, 0];
  // Horizontal bar
  c.fillRect(4, 14, 24, 4, yr, yg, yb);
  // Vertical bar
  c.fillRect(14, 4, 4, 24, yr, yg, yb);
  // Diagonal lines
  c.drawLine(6, 6, 12, 12, yr, yg, yb);
  c.drawLine(26, 6, 20, 12, yr, yg, yb);
  c.drawLine(6, 26, 12, 20, yr, yg, yb);
  c.drawLine(26, 26, 20, 20, yr, yg, yb);
  fs.writeFileSync(path.join(effectsDir, 'sparkle.png'), writePNG(32, 32, c.pixels));
  console.log('✓ effects/sparkle.png');
}

// thought-bubble
{
  const c = createCanvas(32, 32);
  // White circle (approximated with filled squares)
  const cx = 16, cy = 16, radius = 12;
  for (let y = 0; y < 32; y++) {
    for (let x = 0; x < 32; x++) {
      const d = Math.sqrt((x - cx) ** 2 + (y - cy) ** 2);
      if (d <= radius) c.setPixel(x, y, 220, 220, 220);
      else if (d <= radius + 1.5) c.setPixel(x, y, 180, 180, 180);
    }
  }
  // "?" — just a simple 2-pixel question mark
  c.fillRect(13, 8, 6, 2, 30, 30, 40);  // top bar of ?
  c.fillRect(17, 10, 2, 4, 30, 30, 40); // right side
  c.fillRect(15, 14, 2, 2, 30, 30, 40); // curve end (middle)
  // dot
  c.fillRect(15, 20, 2, 2, 30, 30, 40);
  fs.writeFileSync(path.join(effectsDir, 'thought-bubble.png'), writePNG(32, 32, c.pixels));
  console.log('✓ effects/thought-bubble.png');
}

console.log('\nAll assets generated successfully!');
