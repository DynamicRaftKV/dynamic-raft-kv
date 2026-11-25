'use strict';
(()=>{
 let frames=[],index=0,timer=null;
 const make=(tag,text,cls='')=>{const el=document.createElement(tag);el.textContent=text;el.className=cls;return el;};
 function pause(){clearInterval(timer);timer=null;$('process-play').textContent='Play';}
 function render(){
  const f=frames[index],host=$('process-diagram');host.replaceChildren();
  const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 600 480');svg.setAttribute('aria-hidden','true');
  const points={};
  f.nodes.forEach((n,i)=>{const angle=2*Math.PI*i/f.nodes.length-Math.PI/2;points[n.id]={x:300+205*Math.cos(angle),y:240+165*Math.sin(angle)};});
  const defs=document.createElementNS(svg.namespaceURI,'defs'),marker=document.createElementNS(svg.namespaceURI,'marker');marker.id='process-arrow';marker.setAttribute('viewBox','0 0 10 10');marker.setAttribute('refX','9');marker.setAttribute('refY','5');marker.setAttribute('markerWidth','6');marker.setAttribute('markerHeight','6');marker.setAttribute('orient','auto');
  const arrow=document.createElementNS(svg.namespaceURI,'path');arrow.setAttribute('d','M 0 0 L 10 5 L 0 10 z');arrow.setAttribute('fill','#167953');marker.append(arrow);defs.append(marker);svg.append(defs);
  for(const msg of f.messages){const a=points[msg.from],b=points[msg.to];if(!a||!b)continue;const dx=b.x-a.x,dy=b.y-a.y,len=Math.hypot(dx,dy),line=document.createElementNS(svg.namespaceURI,'line');for(const [key,value] of Object.entries({x1:a.x+dx/len*58,y1:a.y+dy/len*48,x2:b.x-dx/len*58,y2:b.y-dy/len*48}))line.setAttribute(key,value);line.setAttribute('marker-end','url(#process-arrow)');svg.append(line);}
  host.append(svg);
  for(const n of f.nodes){const p=points[n.id],node=make('div','',`process-node ${n.role} ${n.online?'':'stopped'}`);node.style.left=p.x/6+'%';node.style.top=p.y/4.8+'%';node.append(make('strong',n.id),make('span',n.online?n.role:'Offline'),make('small',`Term ${n.term} · Last ${n.log}`),make('small',`Commit ${n.commit} · Applied ${n.applied}`));if(n.snapshot)node.append(make('small',`Snapshot ${n.snapshot}`));host.append(node);}
  $('process-position').textContent=`STEP ${index+1} / ${frames.length}`;$('process-title').textContent=f.title;$('process-detail').textContent=f.detail;$('process-quorum').textContent=f.rule;$('process-result').textContent=f.result;
  $('process-messages').replaceChildren(...f.messages.map(m=>make('li',`${m.from} → ${m.to}: ${m.label}`)));
  $('process-step').value=index;$('process-back').disabled=index===0;$('process-next').disabled=index===frames.length-1;
 }
 function load(){pause();frames=raftLesson($('process-scenario').value,Number($('process-count').value));index=0;$('process-step').max=frames.length-1;render();}
 $('process-scenario').onchange=load;$('process-count').onchange=load;$('process-speed').onchange=pause;
 $('process-back').onclick=()=>{pause();index=Math.max(0,index-1);render();};
 $('process-next').onclick=()=>{pause();index=Math.min(frames.length-1,index+1);render();};
 $('process-step').oninput=()=>{pause();index=Number($('process-step').value);render();};
 $('process-reset').onclick=load;
 $('process-play').onclick=()=>{if(timer){pause();return;}if(index===frames.length-1){index=0;render();}$('process-play').textContent='Pause';timer=setInterval(()=>{if(document.hidden||$('view-architecture').hidden){pause();return;}index++;render();if(index===frames.length-1)pause();},Number($('process-speed').value));};
 document.addEventListener('visibilitychange',()=>{if(document.hidden)pause();});
 $('process-observe').onclick=async()=>{
  $('process-observe').disabled=true;$('process-observations').replaceChildren();$('process-observation-status').textContent='Reading node status…';
  try{const rows=await request('cluster/status');const summary=summarizeCluster(rows);$('process-observation-status').textContent=`Observed ${new Date().toLocaleTimeString()}. ${summary.message}`;
   for(const row of rows){const card=make('article','','process-observation'),s=row.status;card.append(make('strong',row.peer.id),make('p',s?`${summary.leader&&!summary.leader.status.peers.some(p=>p.id===row.peer.id)&&!(summary.leader.status.outgoing_voters||[]).some(p=>p.id===row.peer.id)?'Outside current membership · ':''}${s.role} · term ${s.term}`:'Unavailable'),make('p',s?`Commit ${s.commit_index} · Applied ${s.last_applied}`:row.error||'No status'));$('process-observations').append(card);}
  }catch(e){$('process-observation-status').textContent=`Observation unavailable: ${e.message}. Previous observations cleared.`;}
  finally{$('process-observe').disabled=false;}
 };
 load();
})();
