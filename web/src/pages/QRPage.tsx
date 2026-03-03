import { useState, useEffect } from 'react';

export default function QRPage() {
  const [qrData, setQrData] = useState<string | null>(null);
  const [status, setStatus] = useState('WAITING FOR QR CODE...');

  useEffect(() => {
    const es = new EventSource('/dashboard/qr/stream');

    es.onmessage = (e) => {
      if (e.data && e.data.trim()) {
        setQrData(e.data.trim());
        setStatus('SCAN WITH WHATSAPP');
      }
    };

    es.onerror = () => {
      setStatus('QR STREAM UNAVAILABLE');
    };

    return () => es.close();
  }, []);

  return (
    <div style={{ padding: '24px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '16px' }}>
      <h2 style={{ fontSize: '10px', color: '#c9d1d9' }}>WHATSAPP QR</h2>
      <div className="rpg-panel" style={{ padding: '24px', textAlign: 'center' }}>
        {qrData ? (
          <img src={qrData} alt="WhatsApp QR Code" className="pixel-art" style={{ width: '256px', height: '256px', imageRendering: 'pixelated' }} />
        ) : (
          <div style={{ width: '256px', height: '256px', display: 'flex', alignItems: 'center', justifyContent: 'center', border: '2px dashed #30363d' }}>
            <span style={{ color: '#484f58', fontSize: '7px' }}>NO QR DATA</span>
          </div>
        )}
        <p style={{ marginTop: '16px', fontSize: '7px', color: '#8b949e' }}>{status}</p>
      </div>
    </div>
  );
}
