'use strict';
let demonstrationBusy=false, processControls=false;
const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
function evidence(text){$('test-evidence').append(clusterText('li',text));}
async function freshLeader(){
 while(clusterLoading)await delay(50);
 await refreshCluster();
 const leader=clusterLeader();
 if(!leader)throw new Error('No observed leader. Wait for election and retry.');
 if(leader.status.membership_pending)throw new Error('Wait for stable membership before testing.');
 return leader;
}
async function checkedCommand(operation,key,value,expected=200){
 const leader=await freshLeader();
 const response=await fetch('/demo/api/cluster/command',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({node:leader.peer.id,operation,key,value}),signal:AbortSignal.timeout(10000)});
 const result=await response.json();
 if(response.status!==expected)throw new Error(`${operation}: expected HTTP ${expected}, got ${response.status}. ${result.error||'Outcome may be unknown; inspect status.'}`);
 return result;
}
async function waitObserved(check,description){
 const deadline=Date.now()+20000;
 while(Date.now()<deadline){while(clusterLoading)await delay(50);await refreshCluster();if(check())return;await delay(300);}
 throw new Error(description+' did not finish within 20 seconds.');
}
function setDemoBusy(busy){
 demonstrationBusy=busy;
 document.querySelectorAll('.demo-test-actions button,#create-node,#view-cluster form button').forEach(b=>b.disabled=busy);
 if(!busy){$('create-node').disabled=!processControls;$('test-restart').disabled=!processControls;}
}
async function processAction(id,action){
 if(demonstrationBusy||clusterBusy)return;
 if(action==='crash'&&!confirm(`Really kill ${id}? This may temporarily remove quorum. Data and membership are retained; use Restart process to recover.`))return;
 setDemoBusy(true);
 try{await request('cluster/process',{action,id});toast(`${id}: ${action} completed`);}
 catch(e){toast(e.message,true);}
 finally{await refreshCluster();setDemoBusy(false);}
}
$('create-node').onclick=async()=>{
 if(demonstrationBusy||clusterBusy)return;
 setDemoBusy(true);
 try{
  const peer=await request('cluster/process',{action:'create'});
  await refreshCluster();$('cluster-member').value=peer.id;$('cluster-action').value='join';
  $('node-manager-status').textContent=`Created ${peer.id} · HTTP ${peer.http_address} · gRPC ${peer.grpc_address}. Selected below: click Apply membership change to join as learner.`;
  toast('Process started. Join as learner next; it is not a voter yet.');
 }catch(e){$('node-manager-status').textContent=e.message+' Refresh status before retrying: a process may already have been created.';}
 finally{await refreshCluster();setDemoBusy(false);}
};
async function runDemonstration(kind){
 if(demonstrationBusy||clusterBusy)return;
 const selected=$('cluster-member').value;
 if(kind==='restart'&&!confirm('This test really kills and restarts the selected console-created node. Continue?'))return;
 setDemoBusy(true);$('test-evidence').replaceChildren();$('test-state').textContent='Running';$('test-state').className='status-tag';
 const key='demo-test-'+crypto.randomUUID(),value='verified-'+Date.now();
 let stopped=false;
 try{
  const leader=await freshLeader();
  if(kind==='restart'){
   const row=clusterRows.find(r=>r.peer.id===selected),member=leader.status.peers.find(p=>p.id===selected);
   if(!row?.managed||!row.status||!member)throw new Error('Select a reachable, admitted console-created node in Membership first.');
   if(row.status.last_applied<leader.status.commit_index)throw new Error('Selected node is still catching up.');
   const voters=leader.status.peers.filter(p=>!p.learner),reachable=new Set(clusterRows.filter(r=>r.status&&r.peer.id!==selected).map(r=>r.peer.id));
   if(voters.filter(p=>reachable.has(p.id)).length<Math.floor(voters.length/2)+1)throw new Error('Not enough other voters reachable for this restart test.');
  }
  await checkedCommand('PUT',key,value);evidence('PASS · PUT committed for '+key);
  const got=await checkedCommand('GET',key);if(!got.found||got.value!==value)throw new Error('Read did not match acknowledged write.');
  evidence('PASS · GET returned the exact value.');
  if(kind==='replication'||kind==='restart'){
   const l=await freshLeader(),target=l.status.commit_index,ids=l.status.peers.map(p=>p.id);
   if(kind==='restart'){
    await request('cluster/process',{action:'crash',id:selected});stopped=true;
    await waitObserved(()=>!clusterRows.find(r=>r.peer.id===selected)?.status,'Offline observation');evidence('PASS · Selected process is offline.');
    await request('cluster/process',{action:'restart',id:selected});stopped=false;
   }
   await waitObserved(()=>ids.every(id=>{const s=clusterRows.find(r=>r.peer.id===id)?.status;return s&&s.last_applied>=target;}),'Applied-index catch-up');
   evidence(`PASS · All ${ids.length} admitted nodes applied through index ${target}. This checks replication progress, not independent state hashes.`);
   const result=await checkedCommand('GET',key);if(!result.found||result.value!==value)throw new Error('Retained value did not match.');
   evidence('PASS · Retained value verified through consensus.');
  }
  await checkedCommand('DELETE',key);await checkedCommand('GET',key,undefined,404);evidence('PASS · DELETE committed and GET returned 404. Test key cleaned up.');
  $('test-state').textContent='Passed · '+new Date().toLocaleTimeString();$('test-state').className='status-tag passed';
 }catch(e){evidence('FAIL · '+e.message);evidence('Test key: '+key+'. A timeout does not prove the operation was rejected.');$('test-state').textContent='Failed';$('test-state').className='status-tag failed';}
 finally{
  if(stopped){try{await request('cluster/process',{action:'restart',id:selected});evidence('Recovery · Process restarted after interrupted test.');}catch(e){evidence('Recovery needed · '+e.message)}}
  await refreshCluster();setDemoBusy(false);
 }
}
$('test-crud').onclick=()=>runDemonstration('crud');
$('test-replication').onclick=()=>runDemonstration('replication');
$('test-restart').onclick=()=>runDemonstration('restart');
request('cluster/capabilities').then(c=>{processControls=c.local_processes;$('create-node').disabled=!processControls;$('test-restart').disabled=!processControls;$('node-manager-status').textContent=processControls?'Local process controls enabled. Each new node receives two available loopback ports.':'Process controls are disabled. Start the console with --demo-node-seed configs/local-three-node.json and --demo-node-dir data/demo-managed.';}).catch(e=>{$('node-manager-status').textContent=e.message;});
