import { Application, AnimatedSprite, Spritesheet } from 'pixi.js';
import type { AgentState } from '../types/agent';

export function createCatSprite(_app: Application, spritesheet: Spritesheet) {
  // Use idle as default
  const sprite = new AnimatedSprite(spritesheet.animations['idle']);
  
  // Base configuration
  sprite.anchor.set(0.5, 1.0);
  sprite.position.set(200, 280);
  sprite.scale.set(3);
  sprite.animationSpeed = 0.05;
  sprite.zIndex = 2;
  
  // Set texture to nearest neighbor for pixel art
  // In PixiJS v8, texture.source.scaleMode is 'nearest'
  for (const texture of Object.values(spritesheet.textures)) {
    texture.source.scaleMode = 'nearest';
  }
  
  sprite.play();
  
  const setAnimationState = (state: AgentState) => {
    // Only update if animation exists
    if (!spritesheet.animations[state]) {
      console.warn(`Animation for state '${state}' not found`);
      return;
    }
    
    // Set frames
    sprite.textures = spritesheet.animations[state];
    
    // Set speeds
    switch (state) {
      case 'working': sprite.animationSpeed = 0.12; break;
      case 'idle': sprite.animationSpeed = 0.05; break;
      case 'error': sprite.animationSpeed = 0.18; break;
      case 'thinking': sprite.animationSpeed = 0.08; break;
      case 'success': sprite.animationSpeed = 0.20; break;
      default: sprite.animationSpeed = 0.05; break;
    }
    
    sprite.play();
  };
  
  return { sprite, setAnimationState };
}
