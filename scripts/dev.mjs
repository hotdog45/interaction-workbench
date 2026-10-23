import http from 'node:http';
import fs from 'node:fs/promises';
import path from 'node:path';
import {spawn} from 'node:child_process';
import {root,buildUI} from './build-ui.mjs';
await buildUI();let child;
async function ready(){try{const r=await fetch('http://127.0.0.1:4738/demo-api/health',{signal:AbortSignal.timeout(800)});if(!r.ok)throw new Error('occupied');const data=await r.json();if(data.backend!=='Go'||data.mode!=='fixture'||data.version!=='3')throw new Error('occupied');return true}catch(e){if(e.message==='occupied')throw new Error('Port 4738 is occupied by a different service. Stop it or use another environment.');return false}}
if(!await ready()){child=spawn('go',['run','.'],{cwd:path.join(root,'services/api'),stdio:'inherit'});child.on('error',()=>{console.error('Install Go 1.23+ to run the fixture backend.');process.exitCode=1});}else console.log('Using the already running local fixture API.');
let apiReady=false;
for(let attempt=0;attempt<240;attempt++){if(await ready()){apiReady=true;break};if(child&&child.exitCode!==null)break;if(attempt%20===0)console.log('Waiting for the local Go fixture API...');await new Promise(resolve=>setTimeout(resolve,250));}
if(!apiReady){child?.kill('SIGINT');throw new Error('The fixture API did not become ready. Check Go and port 4738.');}
const output=path.join(root,'dist');const mime={'.html':'text/html; charset=utf-8','.js':'text/javascript; charset=utf-8','.css':'text/css; charset=utf-8'};
const server=http.createServer(async(req,res)=>{res.setHeader('Cache-Control','no-store');res.setHeader('X-Content-Type-Options','nosniff');const u=new URL(req.url,'http://local');if(u.pathname.startsWith('/demo-api/')){if(!['GET','POST'].includes(req.method)){res.writeHead(405);res.end();return};const upstream=http.request({hostname:'127.0.0.1',port:4738,path:u.pathname,method:req.method,headers:{'Content-Type':'application/json',...(req.headers.origin?{Origin:req.headers.origin}:{})}},r=>{res.writeHead(r.statusCode,{'Content-Type':r.headers['content-type']||'application/json'});r.pipe(res)});upstream.setTimeout(12000,()=>upstream.destroy());upstream.on('error',()=>{if(!res.headersSent)res.writeHead(503,{'Content-Type':'application/json'});res.end('{"error":"fixture_service_unavailable"}')});req.on('aborted',()=>upstream.destroy());req.pipe(upstream);return};if(!['GET','HEAD'].includes(req.method)){res.writeHead(405);res.end();return};try{const name=u.pathname==='/'?'index.html':decodeURIComponent(u.pathname.slice(1)),target=path.resolve(output,name);if(!target.startsWith(output+path.sep))throw new Error('invalid_path');const data=await fs.readFile(target);res.writeHead(200,{'Content-Type':mime[path.extname(target)]||'application/octet-stream'});res.end(req.method==='HEAD'?undefined:data)}catch{res.writeHead(404);res.end('Not found')}});
server.listen(4322,'127.0.0.1',()=>console.log('Local demo UI: http://localhost:4322/'));
server.on('error',e=>{console.error(e.message);child?.kill('SIGINT');process.exitCode=1});
function stop(){server.close();child?.kill('SIGINT');process.exitCode=0}process.on('SIGINT',stop);process.on('SIGTERM',stop);
