import { useEffect, useRef, useState } from 'react';
import { Application, Sprite, Assets } from 'pixi.js';
import type { AgentState } from '../types/agent';
import { createCatSprite } from './CatSprite';

interface RoomSceneProps {
  catState: AgentState;
}

export default function RoomScene({ catState }: RoomSceneProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [appReady, setAppReady] = useState(false);
  const appRef = useRef<Application | null>(null);
  const catControllerRef = useRef<{ setAnimationState: (state: AgentState) => void } | null>(null);
  
  useEffect(() => {
    if (!canvasRef.current) return;
    
    const app = new Application();
    appRef.current = app;
    let destroyed = false;
    
    app.init({
      canvas: canvasRef.current,
      width: 640,
      height: 480,
      antialias: false,
      roundPixels: true,
      backgroundColor: 0x0d1117,
    }).then(async () => {
      if (destroyed) return;
      
      // Enable sorting
      app.stage.sortableChildren = true;
      
      // Load assets
      const [roomBg, deskBack, deskFront, spritesheet, zzz, alert, sparkle, thought] = await Promise.all([
        Assets.load('/dashboard/sprites/room-bg.png'),
        Assets.load('/dashboard/sprites/desk-back.png'),
        Assets.load('/dashboard/sprites/desk-front.png'),
        Assets.load('/dashboard/sprites/cat-spritesheet.json'),
        Assets.load('/dashboard/sprites/effects/zzz-bubble.png'),
        Assets.load('/dashboard/sprites/effects/alert-bubble.png'),
        Assets.load('/dashboard/sprites/effects/sparkle.png'),
        Assets.load('/dashboard/sprites/effects/thought-bubble.png'),
      ]);
      
      if (destroyed) return;
      
      // Add background layers
      const bgSprite = new Sprite(roomBg);
      bgSprite.zIndex = 0;
      app.stage.addChild(bgSprite);
      
      const deskBackSprite = new Sprite(deskBack);
      deskBackSprite.zIndex = 1;
      app.stage.addChild(deskBackSprite);
      
      const deskFrontSprite = new Sprite(deskFront);
      deskFrontSprite.zIndex = 3;
      app.stage.addChild(deskFrontSprite);
      
      // Add Cat
      const { sprite: catSprite, setAnimationState } = createCatSprite(app, spritesheet);
      catControllerRef.current = { setAnimationState };
      app.stage.addChild(catSprite);
      // Add Effects
      const effectSprite = new Sprite();
      effectSprite.zIndex = 4;
      effectSprite.position.set(200, 200); // Above cat head
      effectSprite.anchor.set(0.5, 1.0);
      app.stage.addChild(effectSprite);
      
      const effectsMap: Record<string, any> = {
        idle: zzz,
        error: alert,
        success: sparkle,
        thinking: thought
      };

      const updateState = (newState: AgentState) => {
        setAnimationState(newState);
        if (effectsMap[newState]) {
          effectSprite.texture = effectsMap[newState];
          effectSprite.visible = true;
        } else {
          effectSprite.visible = false;
        }
      };

      catControllerRef.current = { setAnimationState: updateState };
      
      // Call initial state
      updateState(catState);
      
      setAppReady(true);
    });
    
    return () => {
      destroyed = true;
      app.destroy(true);
      appRef.current = null;
      catControllerRef.current = null;
    };
  }, []); // Run only once to initialize Pixi
  
  // Update animation state when prop changes
  useEffect(() => {
    if (appReady && catControllerRef.current) {
      catControllerRef.current.setAnimationState(catState);
    }
  }, [catState, appReady]);

  return (
    <div style={{ position: 'relative' }}>
      <canvas 
        ref={canvasRef} 
        className="pixel-art" 
        style={{ width: '100%', height: 'auto', display: 'block' }} 
      />
    </div>
  );
}
