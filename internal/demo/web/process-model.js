'use strict';
// Scripted teaching scenarios, not a second Raft implementation or event trace.
function raftLesson(kind, count=3){
 if(![3,5,7].includes(count))throw new Error('Choose 3, 5 or 7 example voters.');
 const majority=Math.floor(count/2)+1;
 let nodes=Array.from({length:count},(_,i)=>({id:`N${i+1}`,role:i?'follower':'leader',term:1,log:1,commit:1,applied:1,online:true,learner:false,snapshot:0}));
 const frames=[];
 function frame(title,detail,patches={},messages=[],result='No client result',rule=`${majority} of ${count} voters required`){
  nodes=nodes.map(n=>({...n,...(patches[n.id]||{})}));
  frames.push({title,detail,nodes:nodes.map(n=>({...n})),messages,result,rule});
 }
 const patchAll=values=>Object.fromEntries(nodes.map(n=>[n.id,values]));
 const fan=(from,targets,label)=>targets.map(to=>({from,to,label}));
 const followers=nodes.slice(1).map(n=>n.id);
 const quorumFollowers=followers.slice(0,majority-1);
 if(kind==='election'){
  frame('Followers wait for heartbeats','Start with empty logs. Randomized election timeouts help avoid repeated split votes.',patchAll({role:'follower',term:0,log:0,commit:0,applied:0}));
  frame('Persist term and vote','N1 times out, increases its term and persists its vote for itself before sending RequestVote.',{N1:{role:'candidate',term:1}},fan('N1',followers,'RequestVote'));
  frame('A majority grants votes','Eligible voters check the candidate’s log and persist their votes before replying. One vote per voter per term.',Object.fromEntries(quorumFollowers.map(id=>[id,{term:1}])),quorumFollowers.map(id=>({from:id,to:'N1',label:'Vote granted'})));
  frame('Leader appends a no-op','With a majority, N1 becomes leader and durably appends an entry in its current term.',{N1:{role:'leader',log:1}},fan('N1',followers,'AppendEntries'));
  frame('Commit the current-term entry','After a durable majority match, the no-op commits. Heartbeats communicate progress to followers.',patchAll({term:1,log:1,commit:1,applied:1}),fan('N1',followers,'Commit index 1'),'Leader established');
 }else if(kind==='write'||kind==='read'){
  const read=kind==='read';
  frame(read?'Client requests GET':'Client requests PUT',read?'This implementation sends reads through the replicated log. It does not use the optional ReadIndex optimization.':'The HTTP request reaches the leader. A follower redirects the client to its known leader.');
  frame('Leader persists entry 2','The leader appends to its WAL before replication. Persisted does not yet mean committed.',{N1:{log:2}});
  frame('Followers persist and acknowledge','AppendEntries carries entry 2. These followers persist it before replying.',Object.fromEntries(quorumFollowers.map(id=>[id,{log:2}])),fan('N1',quorumFollowers,'Append entry 2'));
  frame('A majority makes entry 2 committed','Entry 2 is in the leader’s current term and stored on a voter majority. The leader advances its commit index.',{N1:{commit:2}},quorumFollowers.map(id=>({from:id,to:'N1',label:'Durable ACK'})));
  frame('Apply and answer the client',read?'The state machine evaluates GET in log order and returns the value.':'The state machine applies PUT in order. Only then does the caller receive success.',{N1:{applied:2}},[],read?'GET returns the ordered value':'PUT acknowledged');
  frame('Followers learn the commit index','Later replication communicates the commit. Followers apply in order; this can happen before or after the client receives its response.',patchAll({log:2,commit:2,applied:2}),fan('N1',followers,'Commit index 2'),read?'GET completed':'PUT completed');
 }else if(kind==='quorum'){
  frame('Begin with committed data','All voters contain and have applied entry 1.');
  const offline=nodes.slice(majority-1).map(n=>n.id);
  frame('Lose a voter majority','Too few voters remain reachable. A leader label and healthy HTTP endpoint do not prove quorum.',Object.fromEntries(offline.map(id=>[id,{online:false}])));
  frame('A proposal can still reach the WAL','The isolated leader may durably append entry 2. It cannot commit it without a voter majority.',{N1:{log:2}},[],'Waiting for quorum');
  frame('The caller times out','Commit and applied indices remain at 1. The outcome is unknown: cancellation does not erase a durable proposal.',{},[],'Timeout · outcome unknown');
  frame('Restore communication','The same leader resumes replication in this example. Other executions may elect a new leader and repair an uncommitted suffix.',patchAll({online:true,log:2}),fan('N1',followers,'Retry AppendEntries'));
  frame('Verify recovery with a consensus operation','Once a majority matches, entry 2 can commit and apply. A successful read verifies recovery; role status alone is insufficient.',patchAll({commit:2,applied:2}),[],'Recovery read succeeds');
 }else if(kind==='failover'){
  frame('A committed prefix survives','Entry 1 is durable and applied on every voter.');
  frame('The leader crashes','N1 stops. It does not leave membership; the surviving voters must still meet the original quorum.',{N1:{online:false}});
  frame('An eligible follower campaigns','N2 persists a higher term and self-vote, then requests votes from surviving voters.',{N2:{term:2,role:'candidate'}},fan('N2',followers.filter(id=>id!=='N2'),'RequestVote'));
  frame('A new leader commits a no-op','The surviving majority elects N2. It commits a term-2 no-op before subsequent operations.',Object.fromEntries(followers.map(id=>[id,{term:2,role:id==='N2'?'leader':'follower',log:2,commit:2,applied:2}])),[],'New leader established');
  frame('Restart and catch up the old leader','N1 recovers durable state, learns the newer term and follows N2. Replication repairs its log; committed data is retained.',{N1:{online:true,role:'follower',term:2,log:2,commit:2,applied:2}},[{from:'N2',to:'N1',label:'Catch up'}],'Retained value verified');
 }else if(kind==='snapshot'){
  frame('Applied entries can be checkpointed','The leader and the other voters have applied entry 8. N2 has been lagging since entry 1.',{...patchAll({log:8,commit:8,applied:8}),N2:{log:1,commit:1,applied:1}});
  frame('Persist a snapshot','The checkpoint includes state, membership and the last included index/term. It excludes uncommitted state.',{N1:{snapshot:8}});
  frame('Compact the covered log prefix','After a durable checkpoint, old covered entries can be discarded. The last index remains 8; it is not a count of retained WAL entries.');
  frame('A lagging follower needs the snapshot','N2 missed the retained prefix. The leader sends snapshot chunks because normal log catch-up cannot supply it.',{N2:{log:1,commit:1,applied:1,snapshot:0}},[{from:'N1',to:'N2',label:'Snapshot chunks'}]);
  frame('Install the complete checkpoint','Only a complete, validated snapshot is installed. State and membership restore at the checkpoint boundary.',{N2:{log:8,commit:8,applied:8,snapshot:8}},[],'Follower caught up');
 }else if(kind==='membership'){
  frame('Start with stable voters','The original voter set remains authoritative while a new process is prepared.');
  nodes.push({id:'L',role:'learner',term:1,log:0,commit:0,applied:0,online:true,learner:true,snapshot:0});
  frame('Admit a learner','The leader proposes admission. The learner does not count toward quorum; the existing voters commit the configuration.',Object.fromEntries(nodes.filter(n=>n.id!=='L').map(n=>[n.id,{log:2,commit:2,applied:2}])));
  frame('Catch up before promotion','The learner receives a snapshot/log suffix. The backend rejects premature promotion.',{L:{log:2,commit:2,applied:2,snapshot:2}},[{from:'N1',to:'L',label:'Snapshot + log'}]);
  const joint=`Old set: ${majority}/${count}; new set: ${Math.floor((count+1)/2)+1}/${count+1}. Both required.`;
  frame('Enter joint consensus','Promotion commits a configuration containing old and new voter sets. Progress must satisfy both majorities.',{...patchAll({log:3,commit:3,applied:3}),L:{learner:false,role:'follower',log:3,commit:3,applied:3}},[],'Promotion in progress',joint);
  frame('Finalize the new configuration','After the joint entry is applied, the leader appends the final configuration. Stable membership now contains one extra voter.',patchAll({log:4,commit:4,applied:4}),[],'Promotion stable',`${Math.floor((count+1)/2)+1} of ${count+1} voters required`);
  frame('Remove through joint consensus','The old set includes L; the new set excludes it. Both sets must satisfy their majority before finalization.',patchAll({log:5,commit:5,applied:5}),[],'Removal in progress',`Old set: ${Math.floor((count+1)/2)+1}/${count+1}; new set: ${majority}/${count}. Both required.`);
  frame('Finalize removal','The final configuration retires L permanently. Removal is different from a crash; its files and process may remain.',{...patchAll({log:6,commit:6,applied:6}),L:{role:'retired',learner:false,log:5,commit:5,applied:5}},[],'Removal stable',`${majority} of ${count} voters required`);
 }else throw new Error('Unknown lesson');
 return frames;
}
if(typeof module!=='undefined')module.exports={raftLesson};
