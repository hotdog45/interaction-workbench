import {build} from 'esbuild';
import fs from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
export const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
export async function buildUI(){await fs.mkdir(path.join(root,'dist'),{recursive:true});await build({entryPoints:[path.join(root,'web/scene.jsx')],outfile:path.join(root,'dist/app.js'),bundle:true,minify:true,format:'esm',target:['es2020'],define:{'process.env.NODE_ENV':'"production"'},legalComments:'eof'});for(const name of ['index.html','base.css','scenes.css'])await fs.copyFile(path.join(root,'web',name),path.join(root,'dist',name));}
if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url))await buildUI();
