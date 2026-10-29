'use strict';
let clusterRows=[], clusterLoading=false, clusterBusy=false;
const clusterLeader=()=>clusterRows.filter(r=>r.status?.role==='leader').sort((a,b)=>b.status.term-a.status.term)[0];
function clusterText(tag,text,className=''){const e=document.createElement(tag);e.textContent=text;e.className=className;return e;}
async function refreshCluster(){
 if(clusterLoading)return;clusterLoading=true;
 try{
  clusterRows=await request('cluster/status');
  const leader=clusterLeader(),status=leader?.status;
  if(!$('view-cluster').hidden){$('connection').textContent=status?'Live cluster connected':'Waiting for leader';$('connection').className='connection '+(status?'online':'offline');}
  $('cluster-leader').textContent=status?.id||'No leader';
  $('cluster-term').textContent=status?'Term '+status.term:'Waiting for quorum / election';
  $('cluster-voters').textContent=status?status.peers.filter(p=>!p.learner).length:'—';
  $('cluster-transition').textContent=status?(status.membership_pending?'Changing':'Stable'):'Unknown';
  $('cluster-updated').textContent='Updated '+new Date().toLocaleTimeString();
  const selected=$('cluster-member').value;$('cluster-member').replaceChildren();$('cluster-nodes').replaceChildren();
  for(const row of clusterRows){
   const p=row.peer,s=row.status,member=status?.peers.find(p=>p.id===row.peer.id);
   const option=document.createElement('option');option.value=p.id;option.textContent=p.id;$('cluster-member').append(option);
   const card=clusterText('article','','cluster-node '+(s?.role==='leader'?'leader':'')+(!s?' offline':''));
   const heading=clusterText('h3',p.id);heading.append(clusterText('span',!s?'Offline':!member&&status?'Not a member':member?.learner?'Learner':s.role,'role'));card.append(heading);
   const dl=document.createElement('dl');for(const [k,v] of [['Term',s?.term??'—'],['Commit index',s?.commit_index??'—'],['Applied index',s?.last_applied??'—'],['Catch-up',s&&status?Math.max(0,status.commit_index-s.last_applied)+' behind':'—']]){dl.append(clusterText('dt',k),clusterText('dd',String(v)));}
   if(row.diagnostics){const d=row.diagnostics;for(const [k,v] of [['Pending requests',d.pending],['Uncommitted entries',d.UncommittedEntries],['Snapshot index',d.SnapshotIndex]])dl.append(clusterText('dt',k),clusterText('dd',String(v)));}
   card.append(dl,clusterText('p',p.http_address,'address'));const metrics=document.createElement('a');metrics.href=p.http_address+'/metrics';metrics.target='_blank';metrics.rel='noopener';metrics.textContent='View metrics ↗';metrics.className='address';card.append(metrics);$('cluster-nodes').append(card);
  }
  if(clusterRows.some(r=>r.peer.id===selected))$('cluster-member').value=selected;else if(clusterRows.some(r=>r.peer.id==='node4'))$('cluster-member').value='node4';
 }catch(e){$('cluster-updated').textContent=e.message;clusterRows=[];$('cluster-leader').textContent='Unavailable';}finally{clusterLoading=false;}
}
async function clusterCommand(command){
 if(clusterBusy)return;
 const leader=clusterLeader();if(!leader){toast('No reachable leader. Refresh status and wait for election.',true);return;}
 clusterBusy=true;document.querySelectorAll('#view-cluster form button').forEach(b=>b.disabled=true);
 $('cluster-result-label').textContent='Waiting for quorum';
 try{const result=await request('cluster/command',{node:leader.peer.id,...command});$('cluster-result').textContent=JSON.stringify(result,null,2);$('cluster-result-label').textContent='Acknowledged · '+command.operation;toast(['join','promote','remove'].includes(command.operation)?'Change acknowledged. Check membership status for completion.':'Command committed and applied.');}
 catch(e){$('cluster-result').textContent=e.message;$('cluster-result-label').textContent='Not acknowledged';toast(e.message,true);}
 finally{clusterBusy=false;document.querySelectorAll('#view-cluster form button').forEach(b=>b.disabled=false);await refreshCluster();}
}
$('cluster-refresh').onclick=refreshCluster;
$('cluster-kv').onsubmit=e=>{e.preventDefault();clusterCommand({operation:$('cluster-operation').value,key:$('cluster-key').value,value:$('cluster-value').value});};
$('cluster-membership').onsubmit=e=>{e.preventDefault();const action=$('cluster-action').value,member=$('cluster-member').value;if(action==='remove'&&!confirm('Remove '+member+' from the cluster? Its ID cannot be reused. This is not a crash simulation.'))return;clusterCommand({operation:action,member});};
refreshCluster();setInterval(()=>{if(!document.hidden&&!$('view-cluster').hidden)refreshCluster();},2000);
