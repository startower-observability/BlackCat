export default function LogBubble({ messages }: { messages: string[] }) {
  if (!messages || messages.length === 0) return null;
  
  const lastMessages = messages.slice(-3);
  
  return (
    <div className="rpg-panel" style={{ position: 'absolute', bottom: 8, left: '50%', transform: 'translateX(-50%)', width: '80%', maxWidth: '500px', display: 'flex', flexDirection: 'column', gap: '8px', zIndex: 10 }}>
      {lastMessages.map((msg, i) => (
        <div key={i} style={{ display: 'flex', gap: '8px', opacity: i === lastMessages.length - 1 ? 1 : 0.6 }}>
          <span style={{ color: '#484f58' }}>&gt;</span>
          <span style={{ color: i === lastMessages.length - 1 ? '#c9d1d9' : '#8b949e', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{msg}</span>
        </div>
      ))}
    </div>
  );
}
