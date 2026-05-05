// Shared swimlane rendering for variant_3 task diagrams.
// Each page must define `window.FLOWS` (object) and `window.WIRING` (function returning array of arrows).
// Each arrow is { from, to, key, dashed?, via?, labelOffset? } where from/to/via reference cell IDs.

(function(){
  const canvas = document.querySelector('.swimlane-canvas');
  const svg = document.getElementById('arrows');
  const labelsLayer = document.getElementById('labels-layer');
  const tooltip = document.getElementById('tooltip');
  if (!canvas || !svg || !labelsLayer || !tooltip) return;

  function rectOf(id){
    const el = document.getElementById(id); if(!el) return null;
    const r = el.getBoundingClientRect(); const p = canvas.getBoundingClientRect();
    return { left:r.left-p.left, right:r.right-p.left, top:r.top-p.top, bottom:r.bottom-p.top,
      cx:r.left-p.left+r.width/2, cy:r.top-p.top+r.height/2, width:r.width, height:r.height };
  }
  function drawLine(x1,y1,x2,y2,opts={}){
    const {dashed=false, via=null} = opts;
    const path = document.createElementNS('http://www.w3.org/2000/svg','path');
    let d = `M ${x1} ${y1}`;
    if (Array.isArray(via)) for (const pt of via) d += ` L ${pt.x} ${pt.y}`;
    else if (via) d += ` L ${via.x} ${via.y}`;
    d += ` L ${x2} ${y2}`;
    path.setAttribute('d', d);
    path.setAttribute('fill','none');
    path.setAttribute('stroke','#2C2C2A');
    path.setAttribute('stroke-width','1.4');
    path.setAttribute('opacity','0.55');
    path.setAttribute('marker-end','url(#arrowhead)');
    if (dashed) path.setAttribute('stroke-dasharray','5 4');
    svg.appendChild(path);
  }
  function drawLabel(x,y,key){
    const flow = window.FLOWS[key]; if(!flow) return;
    const l = document.createElement('div');
    l.className = 'arrow-label';
    l.style.left = x+'px';
    l.style.top = y+'px';
    l.textContent = flow.title;
    l.dataset.flowKey = key;
    l.addEventListener('mouseenter', showTooltip);
    l.addEventListener('mousemove', moveTooltip);
    l.addEventListener('mouseleave', hideTooltip);
    labelsLayer.appendChild(l);
  }
  function showTooltip(e){
    const k = e.currentTarget.dataset.flowKey;
    const f = window.FLOWS[k]; if(!f) return;
    let h = `<div class="tt-title">${f.title}</div>`;
    if (f.meta) h += `<div class="tt-meta">${f.meta}</div>`;
    f.sections.forEach((s,i) => {
      if (s.name && f.sections.length > 1) h += `<div class="tt-section" ${i===0?'style="margin-top:0"':''}>${s.name}</div>`;
      h += '<ul>';
      s.items.forEach(it => h += `<li>${it}</li>`);
      h += '</ul>';
    });
    tooltip.innerHTML = h;
    tooltip.classList.add('visible');
    moveTooltip(e);
  }
  function moveTooltip(e){
    const pad = 14;
    const r = tooltip.getBoundingClientRect();
    let x = e.clientX + pad, y = e.clientY + pad;
    if (x + r.width > window.innerWidth - pad) x = e.clientX - r.width - pad;
    if (y + r.height > window.innerHeight - pad) y = e.clientY - r.height - pad;
    tooltip.style.left = x+'px';
    tooltip.style.top = y+'px';
  }
  function hideTooltip(){ tooltip.classList.remove('visible'); }

  function render(){
    svg.innerHTML = ''; labelsLayer.innerHTML = '';
    const pr = canvas.getBoundingClientRect();
    svg.setAttribute('viewBox', `0 0 ${pr.width} ${pr.height}`);
    svg.setAttribute('width', pr.width);
    svg.setAttribute('height', pr.height);
    const defs = document.createElementNS('http://www.w3.org/2000/svg','defs');
    defs.innerHTML = `<marker id="arrowhead" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M2 1 L8 5 L2 9" fill="none" stroke="#2C2C2A" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.6"/></marker>`;
    svg.appendChild(defs);

    if (typeof window.WIRING === 'function') {
      const arrows = window.WIRING(rectOf);
      for (const a of arrows) {
        if (a.kind === 'output') {
          // single short stub from a single rect.bottom downward + label below
          const r = rectOf(a.from); if (!r) continue;
          drawLine(r.cx, r.bottom + 4, r.cx, r.bottom + 16);
          drawLabel(r.cx, r.bottom + 26, a.key);
          continue;
        }
        const from = rectOf(a.from), to = rectOf(a.to);
        if (!from || !to) continue;
        // default routing: horizontal then vertical
        const dx = to.cx - from.cx, dy = to.cy - from.cy;
        let p1, p2, viaPts = a.via;
        if (!viaPts) {
          if (Math.abs(dx) > 2 && Math.abs(dy) > 2) {
            // L-shape via to.cx, from.cy
            viaPts = { x: to.cx, y: from.cy };
            p1 = { x: from.right + 4, y: from.cy };
            p2 = { x: to.cx, y: to.top - 2 };
          } else if (Math.abs(dy) > Math.abs(dx)) {
            // vertical
            p1 = { x: from.cx, y: from.bottom + 4 };
            p2 = { x: to.cx, y: to.top - 2 };
          } else {
            // horizontal
            p1 = { x: from.right + 4, y: from.cy };
            p2 = { x: to.left - 2, y: to.cy };
          }
        } else {
          p1 = { x: from.right + 4, y: from.cy };
          p2 = { x: to.cx, y: to.top - 2 };
        }
        drawLine(p1.x, p1.y, p2.x, p2.y, { dashed: a.dashed, via: viaPts });
        const lo = a.labelOffset || { x: 0, y: -14 };
        drawLabel((from.cx + to.cx)/2 + lo.x, from.cy + lo.y, a.key);
      }
    }
  }

  function rerender(){ requestAnimationFrame(render); }
  function initialRender(){
    if (document.fonts && document.fonts.ready) document.fonts.ready.then(rerender);
    else setTimeout(rerender, 120);
  }
  if (document.readyState === 'complete') initialRender();
  else window.addEventListener('load', initialRender);
  window.addEventListener('resize', () => setTimeout(rerender, 50));
  if (window.ResizeObserver) new ResizeObserver(rerender).observe(canvas);
})();
