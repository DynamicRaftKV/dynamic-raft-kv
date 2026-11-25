const {test}=require('node:test');
const assert=require('node:assert/strict');
const {summarizeCluster}=require('./web/cluster-model.js');
const peers=[1,2,3].map(id=>({id:String(id),learner:false}));
const row=(id,role,term)=>({peer:{id:String(id)},status:{role,term,peers}});
test('an isolated old leader is not selected over a newer term',()=>{
 assert.equal(summarizeCluster([row(1,'leader',1),row(2,'candidate',2)]).leader,null);
});
test('contradictory same-term leaders are not presented as authoritative',()=>{
 assert.equal(summarizeCluster([row(1,'leader',2),row(2,'leader',2)]).leader,null);
});
test('reachable leader alone does not imply quorum',()=>{
 const s=summarizeCluster([row(1,'leader',2),{peer:peers[1]}]);
 assert.equal(s.available,1);assert.equal(s.required,2);assert.match(s.message,/may time out/);
});
test('joint membership avoids a misleading single-majority summary',()=>{
 const r=row(1,'leader',2);r.status.membership_pending=true;
 assert.match(summarizeCluster([r]).message,/Both voter sets/);
});
test('missing observations clear the leader',()=>assert.equal(summarizeCluster([]).leader,null));
test('automatic promotion waits for a current caught-up learner and stable membership',()=>{
 const {readyToPromote}=require('./web/cluster-model.js');
 const leader=row(1,'leader',3);
 leader.status.peers=[...peers,{id:'4',learner:true}];
 leader.status.commit_index=10;
 const learner=row(4,'follower',3);learner.status.last_applied=9;
 assert.equal(readyToPromote([leader,learner],'4'),false);
 learner.status.last_applied=10;
 assert.equal(readyToPromote([leader,learner],'4'),true);
 leader.status.membership_pending=true;
 assert.equal(readyToPromote([leader,learner],'4'),false);
 leader.status.membership_pending=false;learner.status.term=2;
 assert.equal(readyToPromote([leader,learner],'4'),false);
 assert.equal(readyToPromote([leader],'4'),false);
 assert.equal(readyToPromote([leader,learner],'unknown'),false);
});
test('retired high-term process cannot hide a current leader',()=>{
 const leader=row(1,'leader',3),retired=row(4,'candidate',1000);
 assert.equal(summarizeCluster([leader,retired]).leader,leader);
 leader.status.outgoing_voters=[{id:'4'}];
 assert.equal(summarizeCluster([leader,retired]).leader,null);
});
