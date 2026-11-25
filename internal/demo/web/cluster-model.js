'use strict';
// Status is advisory: reachability from the console does not prove that peers
// can communicate with each other or that a read can currently commit.
function summarizeCluster(rows) {
 const observed=rows.filter(r=>r.status);
 // A retired process can keep campaigning with increasing terms. Only peers
 // in an observed leader's configuration can contradict its leadership.
 // Include outgoing voters during joint consensus.
 const reportedLeaders=observed.filter(r=>r.status.role==='leader');
 const relevantIDs=new Set(reportedLeaders.flatMap(r=>[r.peer.id,...r.status.peers.map(p=>p.id),...(r.status.outgoing_voters||[]).map(p=>p.id)]));
 const relevant=reportedLeaders.length?observed.filter(r=>relevantIDs.has(r.peer.id)):observed;
 const term=Math.max(0,...relevant.map(r=>r.status.term));
 const leaders=observed.filter(r=>r.status.role==='leader'&&r.status.term===term);
 const leader=leaders.length===1?leaders[0]:null;
 if(!leader)return {leader:null,message:leaders.length>1?'Conflicting leader reports. Wait for a fresh observation.':'No leader confirmed at the newest observed term. Waiting for election.'};
 const voters=leader.status.peers.filter(p=>!p.learner);
 const reachable=new Set(observed.map(r=>r.peer.id));
 const available=voters.filter(p=>reachable.has(p.id)).length;
 const required=Math.floor(voters.length/2)+1;
 return {leader,available,required,message:leader.status.membership_pending
  ?'Membership transition in progress. Both voter sets must satisfy their quorum rules.'
  :available<required?`${available} of ${voters.length} voters reachable from this console; ${required} required. Operations may time out.`
  :`${available} of ${voters.length} voters reachable from this console; ${required} required. Only a successful command confirms quorum progress.`};
}
function readyToPromote(rows,id){
 const leader=summarizeCluster(rows).leader,s=leader?.status;
 const member=s?.peers.find(p=>p.id===id),node=rows.find(r=>r.peer.id===id)?.status;
 return !!(s&&!s.membership_pending&&member?.learner&&node&&node.term===s.term&&node.last_applied>=s.commit_index);
}
if(typeof module!=='undefined')module.exports={summarizeCluster,readyToPromote};
