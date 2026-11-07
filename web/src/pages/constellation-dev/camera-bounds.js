// Keep the real galactic disk overscanning the viewport, not shrinking to a framed coin.
// Perspective conservatism covers the far half; pitch and aspect both affect the floor.
export function minimumImmersiveZoom(width,height,pitch){
 const scale=Math.min(Math.max(1,width)*.375,Math.max(1,height)*.59),tilt=Math.max(.22,Math.abs(Math.cos(pitch)));
 return Math.min(9,Math.max(1.3,1.06*Math.hypot(width*.5/scale,height*.515/(scale*tilt))/(1.4*.82)));
}
export function boundCamera(camera,width,height){
 return {...camera,zoom:Math.max(minimumImmersiveZoom(width,height,camera.pitch),Math.min(9,camera.zoom))};
}
