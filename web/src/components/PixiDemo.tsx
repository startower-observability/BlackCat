import { useEffect, useRef } from 'react'
import { Application, Graphics, AnimatedSprite, Texture } from 'pixi.js'

export default function PixiDemo() {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  
  useEffect(() => {
    if (!canvasRef.current) return
    
    const app = new Application()
    let destroyed = false
    
    app.init({
      canvas: canvasRef.current,
      width: 300,
      height: 300,
      antialias: false,
      roundPixels: true,
      backgroundColor: 0x0d1117,
    }).then(() => {
      if (destroyed) return
      
      // Create 3 colored rectangle textures
      const colors = [0xff4444, 0x44ff44, 0x4444ff]
      const textures: Texture[] = colors.map(color => {
        const g = new Graphics()
        g.rect(0, 0, 48, 48).fill(color)
        return app.renderer.generateTexture(g)
      })
      
      const sprite = new AnimatedSprite(textures)
      sprite.x = 126
      sprite.y = 126
      sprite.anchor.set(0.5)
      sprite.animationSpeed = 0.1
      sprite.play()
      
      app.stage.addChild(sprite)
    })
    
    return () => {
      destroyed = true
      app.destroy(true)
    }
  }, [])
  
  return (
    <canvas
      ref={canvasRef}
      style={{ border: '2px solid #30363d', display: 'block' }}
    />
  )
}
