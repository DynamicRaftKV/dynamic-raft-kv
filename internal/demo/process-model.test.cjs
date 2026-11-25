const {test}=require('node:test');
const assert=require('node:assert/strict');
const {raftLesson}=require('./web/process-model.js');
for(const count of [3,5,7]){
 test(`${count}-voter lessons preserve index ordering and independent frames`,()=>{
  for(const kind of ['election','write','read','quorum','failover','snapshot','membership']){
   const frames=raftLesson(kind,count);
   for(const [i,f] of frames.entries())for(const n of f.nodes){
    assert.ok(n.applied<=n.commit&&n.commit<=n.log,`${kind}: ${n.id} invalid indices`);
    const previous=i&&frames[i-1].nodes.find(p=>p.id===n.id);
    if(previous){assert.ok(n.commit>=previous.commit);assert.ok(n.applied>=previous.applied);assert.notEqual(n,previous);}
   }
  }
 });
 test(`${count}-voter minority cannot commit before recovery`,()=>{
  const frames=raftLesson('quorum',count),timeout=frames.find(f=>f.result.startsWith('Timeout'));
  assert.ok(timeout.nodes.filter(n=>n.online).length<Math.floor(count/2)+1);
  assert.equal(timeout.nodes[0].log,2);assert.equal(timeout.nodes[0].commit,1);assert.equal(timeout.nodes[0].applied,1);
  assert.equal(frames.at(-1).nodes[0].commit,2);
 });
 test(`${count}-voter client response follows durability, quorum and apply`,()=>{
  for(const kind of ['write','read']){
   const frames=raftLesson(kind,count),committed=frames[3],answered=frames[4];
   assert.ok(committed.nodes.filter(n=>n.log>=2).length>=Math.floor(count/2)+1);
   assert.equal(committed.nodes[0].applied,1);
   assert.equal(answered.nodes[0].applied,2);
   assert.equal(frames[1].nodes[0].commit,1);
  }
 });
 test(`${count}-voter membership keeps learner outside quorum until joint phase`,()=>{
  const f=raftLesson('membership',count);
  assert.ok(f[1].nodes.at(-1).learner);assert.ok(f[2].nodes.at(-1).learner);
  assert.equal(f[3].nodes.at(-1).learner,false);assert.match(f[3].rule,/Both required/);
  assert.match(f[5].rule,/Both required/);assert.equal(f.at(-1).nodes.at(-1).role,'retired');
 });
}
test('invalid scenario settings fail explicitly',()=>{
 assert.throws(()=>raftLesson('write',4));assert.throws(()=>raftLesson('other',3));
});
