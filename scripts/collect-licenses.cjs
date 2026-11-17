const fs=require('node:fs/promises');
const path=require('node:path');
const cp=require('node:child_process');
async function copyNotice(src,dst){try{await fs.chmod(dst,0o666)}catch(e){if(e.code!=='ENOENT')throw e}await fs.copyFile(src,dst)}
(async()=>{
 const root=process.cwd();
 const active=new Set();
 const out=cp.execFileSync('go',['list','-m','-json','all'],{encoding:'utf8'});
 const modules=out.trim().split(/\r?\n(?=\{)/).map(x=>JSON.parse(x));
 const lines=['# Bajiu DB — Third-party notices','','Frameworks, icons and database drivers retain their own licences. This file does not set the licence of the application’s original code.','','## Go dependencies',''];
 await fs.mkdir(path.join(root,'licenses'),{recursive:true});
 for(const m of modules){if(m.Main||!m.Dir)continue;const files=(await fs.readdir(m.Dir)).filter(n=>/^(licen[cs]e|copying|notice|copyright)([._-].*)?$/i.test(n));if(!files.length){lines.push(`- ${m.Path} ${m.Version}: see its upstream repository for licence notices.`);continue}const key=m.Path.replaceAll('/','_')+'-'+m.Version;active.add(key);await fs.mkdir(path.join(root,'licenses',key),{recursive:true});for(const name of files){const stat=await fs.stat(path.join(m.Dir,name));if(stat.isFile())await copyNotice(path.join(m.Dir,name),path.join(root,'licenses',key,name))}lines.push(`- ${m.Path} ${m.Version}: licences in licenses/${key}/`)}
 lines.push('','## Frontend runtime dependencies','');
 for(const name of ['vue','@vue/shared','@vue/reactivity','@vue/runtime-core','@vue/runtime-dom','@lucide/vue','@wailsio/runtime']){const dir=path.join(root,'frontend','node_modules',name);const pkg=JSON.parse(await fs.readFile(path.join(dir,'package.json'),'utf8'));const key=name.replaceAll('/','_')+'-'+pkg.version;active.add(key);await fs.mkdir(path.join(root,'licenses',key),{recursive:true});for(const n of await fs.readdir(dir)){if(/^licen[cs]e([._-].*)?$/i.test(n))await copyNotice(path.join(dir,n),path.join(root,'licenses',key,n))}if(name==='@wailsio/runtime'){const sdk=modules.find(m=>m.Path==='github.com/wailsapp/wails/v3');if(sdk?.Dir)await copyNotice(path.join(sdk.Dir,'LICENSE'),path.join(root,'licenses',key,'LICENSE'))}lines.push(`- ${name} ${pkg.version}: ${pkg.license}; licences in licenses/${key}/`)}
 lines.push('','## MySQL driver source availability','','This application includes github.com/go-sql-driver/mysql v1.10.1 under MPL-2.0. The unmodified corresponding source archive is included as third_party_sources/mysql-v1.10.1.zip. You may obtain it, including licence text, from https://github.com/go-sql-driver/mysql/tree/v1.10.1. The source archive covers the driver; it does not require treating unrelated application files as MPL-covered source.','','## Go runtime','','The Go runtime licence is included in licenses/Go-LICENSE.','');
 const goroot=cp.execFileSync('go',['env','GOROOT'],{encoding:'utf8'}).trim();await copyNotice(path.join(goroot,'LICENSE'),path.join(root,'licenses','Go-LICENSE'));
 const cache=cp.execFileSync('go',['env','GOMODCACHE'],{encoding:'utf8'}).trim();await fs.mkdir(path.join(root,'third_party_sources'),{recursive:true});await copyNotice(path.join(cache,'cache/download/github.com/go-sql-driver/mysql/@v/v1.10.1.zip'),path.join(root,'third_party_sources','mysql-v1.10.1.zip'));
 await fs.writeFile(path.join(root,'THIRD_PARTY_NOTICES.md'),lines.join('\n'));
 const archive=path.join(root,'.local','license-archive');await fs.mkdir(archive,{recursive:true});
 for(const entry of await fs.readdir(path.join(root,'licenses'),{withFileTypes:true})){if(!entry.isDirectory()||active.has(entry.name))continue;const src=path.resolve(root,'licenses',entry.name);const boundary=path.resolve(root,'licenses')+path.sep;if(!src.startsWith(boundary))throw Error('License archive boundary violation');let dst=path.join(archive,entry.name);try{await fs.access(dst);dst+='-'+Date.now()}catch{}await fs.rename(src,dst)}
 console.log('Collected third-party licences and the unmodified MySQL driver source archive');
})().catch(e=>{console.error(e);process.exitCode=1});
