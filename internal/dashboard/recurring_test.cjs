const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');

async function fixture(){
 class Element{
  constructor(tag='div'){this.tag=tag;this.children=[];this.listeners={};this.value='';}
  append(...items){this.children.push(...items);}
  replaceChildren(...items){this.children=items;}
  addEventListener(name,fn){this.listeners[name]=fn;}
  scrollIntoView(){}
  querySelectorAll(selector){const all=[];const visit=n=>{if(!n||typeof n!=='object')return;if(selector==='input:checked'&&n.tag==='input'&&n.checked)all.push(n);if(selector==='button'&&n.tag==='button')all.push(n);for(const c of n.children||[])visit(c);};for(const c of this.children)visit(c);return all;}
 }
 const nodes=new Map(),calls=[];
 const $=id=>{if(!nodes.has(id))nodes.set(id,new Element());return nodes.get(id);};
 const ctx=vm.createContext({$,D:{owner:true,today:'2026-11-16',list_at:-900},el:tag=>new Element(tag),svgEl:()=>new Element(),icon:()=>'',cap:s=>s,
  meals:()=>['breakfast','lunch','snacks','dinner'],addDays:(date,n)=>{const d=new Date(date+'T12:00:00Z');d.setUTCDate(d.getUTCDate()+n);return d.toISOString().slice(0,10);},
  location:{hash:'',pathname:'/sales.html',search:''},history:{replaceState(){}},addEventListener(){},load:async()=>{},
  fetch:async()=>({ok:true,status:200,json:async()=>[]}),
  send:async(method,url,body)=>{calls.push(body);return body.action==='preview'?{new:5,starts_now:1,waiting:2,preserved_overrides:1,kept:1,existing:0}:{...body.rule,id:7,revision:2};},
 });
 vm.runInContext(fs.readFileSync(path.join(__dirname,'static/recurring.js'),'utf8'),ctx);
 await new Promise(resolve=>setImmediate(resolve));
 return {$,calls,run:code=>vm.runInContext(code,ctx)};
}

test('rule preview is invalidated when settings change',async()=>{
 const f=await fixture();f.$('ruleName').value='Lunches';
 await f.$('rulePreview').onclick();assert.equal(f.$('ruleSave').disabled,false);assert.match(f.$('rulePreviewText').textContent,/1 start selling immediately/);
 f.$('ruleMin').value='31';f.$('ruleForm').listeners.input();
 assert.equal(f.$('ruleSave').disabled,true);
 await f.$('ruleForm').listeners.submit({preventDefault(){}});
 assert.equal(f.calls.filter(c=>c.action==='save').length,0);
});

test('saving uses previewed weekday and meal settings and apply option',async()=>{
 const f=await fixture();f.$('ruleName').value='Lunches';f.$('ruleMin').value='30';f.$('ruleMax').value='60';f.$('ruleApply').checked=true;
 await f.$('rulePreview').onclick();await f.$('ruleForm').listeners.submit({preventDefault(){}});
 const saved=f.calls.find(c=>c.action==='save');assert.ok(saved);assert.deepEqual(Array.from(saved.rule.weekdays),[1,2,3,4,5]);assert.deepEqual(Array.from(saved.rule.meals),['lunch']);assert.equal(saved.rule.before,15);assert.equal(saved.rule.min,30);assert.equal(saved.apply_waiting,true);
});

test('list refresh preserves an unsaved rule form',async()=>{
 const f=await fixture();f.$('ruleName').value='Unsaved changes';f.$('ruleMin').value='31';
 await f.run('loadRecurringRules()');assert.equal(f.$('ruleName').value,'Unsaved changes');assert.equal(f.$('ruleMin').value,'31');
});

test('deleting targets the selected rule revision and explicit waiting-removal choice',async()=>{
 const f=await fixture();f.$('ruleID').value='7';f.$('ruleRevision').value='2';f.$('ruleRemoveWaiting').checked=true;
 await f.$('ruleDelete').onclick();const deleted=f.calls.find(c=>c.action==='delete');assert.equal(deleted.rule.id,7);assert.equal(deleted.rule.revision,2);assert.equal(deleted.cancel_waiting,true);
});
