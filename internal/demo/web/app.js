'use strict';
const $ = id => document.getElementById(id);
let operation = 'PUT', busy = false, savedSnapshot = null, capturedAt = null;
const history = [];
const views = {
 workspace: ['KV workspace', 'Write a key. Read it back. See your Go engine at work.'],
 snapshots: ['Snapshots', 'Capture your data, make a change, then restore the exact state.'],
 implementation: ['Implementation', 'A clear view of what is built, verified, and still ahead.'],
 architecture: ['Architecture', 'See the live demo path and how it fits the planned system.']
};
function showView(name) {
 if (!views[name]) name = 'workspace';
 document.querySelectorAll('.view').forEach(el => el.hidden = el.id !== 'view-' + name);
 document.querySelectorAll('.nav-item').forEach(el => { el.classList.toggle('active', el.dataset.view === name); if (el.dataset.view === name) el.setAttribute('aria-current','page'); else el.removeAttribute('aria-current'); });
 $('page-title').textContent = views[name][0]; $('breadcrumb').textContent = views[name][0]; $('page-description').textContent = views[name][1];
 historyReplace(name);
}
function historyReplace(name) { window.history.replaceState(null, '', '#' + name); }
document.querySelectorAll('[data-view]').forEach(el => el.addEventListener('click', () => showView(el.dataset.view)));
window.addEventListener('hashchange', () => showView(location.hash.slice(1)));
function selectOperation(op) {
 operation = op;
 document.querySelectorAll('.op').forEach(el => { el.classList.toggle('active', el.dataset.op === op); el.setAttribute('aria-pressed', String(el.dataset.op === op)); });
 $('value-field').hidden = op !== 'PUT';
 $('operation-hint').textContent = { PUT: 'Creates a key or overwrites its value. Empty values are supported.', GET: 'Reads the current value without changing state. Missing keys return found = false.', DELETE: 'Removes the key. Deleting a missing key also succeeds.' }[op];
}
document.querySelectorAll('[data-op]').forEach(el => el.addEventListener('click', () => selectOperation(el.dataset.op)));
function toast(message, error = false) {
 const el = $('toast'); clearTimeout(toast.timer); el.textContent = message; el.classList.toggle('error', error); el.hidden = false;
 toast.timer = setTimeout(() => el.hidden = true, 5000);
}
async function request(path, data) {
 if (location.protocol === 'file:') throw new Error('Open http://127.0.0.1:8080/demo/ to use the Go engine. The HTML file cannot run commands on its own.');
 let response;
 try {
  response = await fetch('/demo/api/' + path, { method: data === undefined ? 'GET' : 'POST', headers: data === undefined ? {} : { 'Content-Type':'application/json' }, body: data === undefined ? undefined : JSON.stringify(data), signal: AbortSignal.timeout(10000) });
 } catch (_) { throw new Error('The Go demo server is unavailable. Start it with --demo and keep its terminal running, then refresh this page.'); }
 const result = await response.json();
 if (!response.ok) throw new Error(result.error || 'Request failed');
 return result;
}
async function refresh() {
 try {
  const view = await request('state');
  $('connection').className = 'connection online'; $('connection').textContent = 'Go engine connected';
  const items = Object.entries(view.snapshot.data).sort(([a],[b]) => a < b ? -1 : a > b ? 1 : 0);
  $('key-count').textContent = items.length; $('table-count').textContent = items.length; $('command-count').textContent = view.commands;
  $('empty-state').hidden = items.length !== 0; $('key-table').replaceChildren();
  for (const [key, value] of items) {
   const row = document.createElement('tr'), k = document.createElement('td'), v = document.createElement('td'), action = document.createElement('td'), read = document.createElement('button');
   k.textContent = key; k.title = key; k.className = 'key-cell'; v.textContent = value === '' ? '""' : value; v.title = value; v.className = 'value-cell';
   read.textContent = '↗'; read.className = 'row-read'; read.setAttribute('aria-label','Read key ' + key);
   read.onclick = () => { selectOperation('GET'); $('key').value = key; $('key').focus(); };
   action.append(read); row.append(k,v,action); $('key-table').append(row);
  }
  return view;
 } catch (err) { $('connection').className = 'connection offline'; $('connection').textContent = 'Engine unavailable'; throw err; }
}
function addActivity(op, detail, error = false) {
 history.unshift({op,detail,error,time:new Date()}); if (history.length > 12) history.pop(); $('activity').replaceChildren();
 for (const item of history) {
  const row = document.createElement('div'), tag = document.createElement('span'), text = document.createElement('span'), time = document.createElement('span');
  row.className='activity-row'; tag.className='activity-op' + (item.error ? ' error':''); tag.textContent=item.op; text.className='activity-detail'; text.textContent=item.detail; text.title=item.detail;
  time.className='activity-time'; time.textContent=item.time.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit',second:'2-digit'}); row.append(tag,text,time); $('activity').append(row);
 }
}
async function withBusy(action) {
 if (busy) return; busy=true;
 const controls = [...document.querySelectorAll('#run,#seed,#capture,#restore,#refresh')]; controls.forEach(el => el.disabled = true);
 try { return await action(); } catch(err) { toast(err.message, true); }
 finally { busy=false; controls.forEach(el => el.disabled=false); $('restore').disabled=!savedSnapshot; }
}
async function applyCommand(command) {
 let result;
 try { result = await request('command', command); }
 catch(err) { $('response').textContent = err.message; $('response-status').textContent='Rejected'; addActivity(command.op, err.message, true); throw err; }
 $('response').textContent=JSON.stringify(result,null,2); $('response-status').textContent='Applied';
 addActivity(command.op, command.op === 'GET' ? command.key + (result.found ? ' → ' + JSON.stringify(result.value || '') : ' → not found') : command.key + (command.op === 'PUT' ? ' ← ' + JSON.stringify(command.value || '') : ' removed'));
 await refresh(); return result;
}
$('command-form').addEventListener('submit', event => { event.preventDefault(); const command={op:operation,key:$('key').value}; if(operation==='PUT') command.value=$('value').value; withBusy(() => applyCommand(command)); });
$('refresh').onclick = () => withBusy(async () => { await refresh(); toast('Keyspace refreshed.'); });
$('seed').onclick = () => withBusy(async () => {
 const samples = [['project:name','RaftKV'],['engine:language','Go'],['engine:mode','standalone'],['demo:message','Hello, distributed systems.']];
 const current = await request('state');
 for (const [key,value] of samples) if (!Object.hasOwn(current.snapshot.data,key)) await applyCommand({op:'PUT',key,value});
 toast('Example keys written to the Go engine.');
});
$('capture').onclick = () => withBusy(async () => {
 savedSnapshot = await request('snapshot'); capturedAt=new Date(); $('snapshot-json').textContent=JSON.stringify(savedSnapshot,null,2); $('download').disabled=false;
 $('snapshot-meta').textContent=Object.keys(savedSnapshot.data).length+' keys captured at '+capturedAt.toLocaleTimeString();
 addActivity('SNAPSHOT','Captured '+Object.keys(savedSnapshot.data).length+' keys'); toast('Snapshot captured. You can now change the live data.');
});
$('restore').onclick=() => { if(savedSnapshot) $('restore-dialog').showModal(); };
$('restore-dialog').addEventListener('close', () => { if($('restore-dialog').returnValue==='restore') withBusy(async () => { await request('restore',savedSnapshot); await refresh(); addActivity('RESTORE','Replaced state with '+Object.keys(savedSnapshot.data).length+' captured keys'); toast('Captured state restored.'); }); });
$('download').onclick = () => {
 if (!savedSnapshot) return;
 const url=URL.createObjectURL(new Blob([JSON.stringify(savedSnapshot,null,2)],{type:'application/json'})), link=document.createElement('a');
 link.href=url; link.download='raftkv-snapshot-v1.json'; link.click(); setTimeout(() => URL.revokeObjectURL(url),1000);
};
showView(location.hash.slice(1));
if (location.protocol === 'file:') {
 $('connection').textContent = 'Open the server URL';
 $('connection').className = 'connection offline';
 const notice = document.querySelector('.notice');
 notice.replaceChildren();
 const message = document.createElement('span');
 message.textContent = 'This is the HTML file only. Commands need the running Go server. ';
 const link = document.createElement('a');
 link.href = 'http://127.0.0.1:8080/demo/';
 link.textContent = 'Open the working prototype →';
 link.style.textDecoration = 'underline';
 notice.append(message, link);
 document.querySelectorAll('#run,#seed,#capture,#restore,#refresh').forEach(el => el.disabled = true);
} else {
 refresh().catch(err => toast(err.message,true));
}

// Optional agent access shares the same read-back path as the visible table.
if (document.modelContext?.registerTool) {
 const lifecycle = new AbortController();
 try {
  Promise.resolve(document.modelContext.registerTool({
   name:'inspect_raftkv_keyspace', title:'Inspect RaftKV keyspace',
   description:'Refresh the visible keyspace and return the current standalone Go engine state. Does not modify data.',
   inputSchema:{type:'object',properties:{},additionalProperties:false},
   annotations:{readOnlyHint:true,untrustedContentHint:true},
   async execute(input) {
    if (!input || typeof input !== 'object' || Array.isArray(input) || Object.keys(input).length) throw new Error('Expected an empty object.');
    const view=await refresh(); return {data:view.snapshot.data,commands:view.commands,mode:'standalone'};
   }
  },{signal:lifecycle.signal})).catch(() => {});
 } catch (_) { /* Unsupported preview browsers still use the regular UI. */ }
 window.addEventListener('pagehide',()=>lifecycle.abort(),{once:true});
}
