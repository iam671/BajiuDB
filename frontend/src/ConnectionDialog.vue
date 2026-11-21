<script setup lang="ts">
import { ref, nextTick, onMounted,computed } from 'vue'
import {engineOf,engineLabel,mysqlFamily} from './databaseEngines'
import {api} from './api'
import DatabaseEngineIcon from './DatabaseEngineIcon.vue'
import { X, LoaderCircle, FolderOpen, Eye, EyeOff, CheckCircle2, AlertCircle } from '@lucide/vue'
import type { ConnectionInput, TestResult, ConnectionGroup } from './api'
import { APP_ICON } from './api'
import {vDraggableDialog} from './draggableDialog'

const props = defineProps<{
  model:ConnectionInput; busy:string; errors:Record<string,string>; result:TestResult|null;
  error:string; editing:boolean; connectOnSave:boolean; fatal:boolean;
  groups:ConnectionGroup[];
}>()
const emit = defineEmits<{
  save:[]; cancel:[]; test:[]; password:[]; forget:[]; chooseCA:[];chooseSshKey:[];
}>()
const tab = ref('general')
const visiblePassword = ref(false)
const allTabs = [{id:'general',label:'常规'},{id:'advanced',label:'高级'},{id:'databases',label:'数据库'},{id:'ssl',label:'SSL'},{id:'ssh',label:'SSH'},{id:'http',label:'HTTP'}]
const tabs=computed(()=>allTabs.filter(t=>engineOf(props.model)==='sqlite'?['general','advanced'].includes(t.id):mysqlFamily(props.model)||!['ssh','http'].includes(t.id)))
const pickerError=ref('')
async function chooseSQLite(){try{const path=await api.SelectSQLiteFile();if(path)props.model.filePath=path;pickerError.value=''}catch(e){pickerError.value=String(e)}}
function focusFirst(){nextTick(()=>document.getElementById('connection-name')?.focus())}
function trap(e:KeyboardEvent){const nodes=Array.from((e.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]')).filter(el=>el.getClientRects().length);const first=nodes[0],last=nodes[nodes.length-1];if(e.shiftKey&&document.activeElement===first){e.preventDefault();last?.focus()}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first?.focus()}}
function selectTab(id:string){tab.value=id;nextTick(()=>document.getElementById('connection-tabs-'+id)?.focus())}
function tabKey(e:KeyboardEvent){const list=tabs.value,i=list.findIndex(t=>t.id===tab.value);if(e.key==='ArrowRight'||e.key==='ArrowLeft'){e.preventDefault();selectTab(list[(i+(e.key==='ArrowRight'?1:list.length-1))%list.length]!.id)}}
function showValidation(){tab.value='general';focusFirst()}
defineExpose({showValidation})
onMounted(focusFirst)
</script>

<template>
  <div class="connection-overlay">
    <section v-draggable-dialog class="connection-dialog" role="dialog" aria-modal="true" aria-labelledby="connection-dialog-title" @keydown.tab="trap">
      <header class="dialog-titlebar connection-drag-handle" title="拖动标题栏移动，双击标题栏或 Alt+Home 居中"><DatabaseEngineIcon :engine="model.engine" :size="16"/><h2 id="connection-dialog-title">{{engineLabel(model)}} - {{connectOnSave?'打开连接':editing?'编辑连接':'新建连接'}}</h2><button type="button" class="window-close" aria-label="关闭连接窗口" @click="emit('cancel')" :disabled="!!busy"><X :size="16"/></button></header>
      <form class="dialog-form" @submit.prevent="emit('save')">
        <div class="connection-tabbar" role="tablist" aria-label="连接设置分页" @keydown="tabKey">
          <button v-for="t in tabs" :key="t.id" type="button" role="tab" :id="'connection-tabs-'+t.id" :aria-controls="'connection-panel-'+t.id" :aria-selected="tab===t.id" :tabindex="tab===t.id?0:-1" :class="{active:tab===t.id}" @click="tab=t.id" :disabled="!!busy">{{t.label}}</button>
        </div>
        <fieldset class="dialog-content" :disabled="!!busy||fatal">
          <section v-show="tab==='general'" role="tabpanel" id="connection-panel-general" aria-labelledby="connection-tabs-general">
            <div class="connection-route" aria-hidden="true"><div><img :src="APP_ICON" alt="" width="39" height="39"/><span>Bajiu DB</span></div><hr/><div><DatabaseEngineIcon :engine="model.engine" :size="42"/><span>{{engineLabel(model)}}</span></div></div>
            <div class="native-form-row name-row"><label for="connection-name">连接名：</label><div class="native-field"><input id="connection-name" v-model="model.name" maxlength="80" autocomplete="off" :aria-invalid="!!errors.name"/><span v-if="errors.name" class="field-error">{{errors.name}}</span></div></div>
            <div class="native-form-row" v-if="engineOf(model)!=='sqlite'"><label for="connection-host">主机：</label><div class="native-field"><input id="connection-host" v-model="model.host" maxlength="253" spellcheck="false" :aria-invalid="!!errors.host"/><span v-if="errors.host" class="field-error">{{errors.host}}</span></div></div>
            <div class="native-form-row" v-if="engineOf(model)!=='sqlite'"><label for="connection-port">端口：</label><div class="native-field"><input id="connection-port" class="short-input" v-model.number="model.port" type="number" min="1" max="65535" :aria-invalid="!!errors.port"/><span v-if="errors.port" class="field-error">{{errors.port}}</span></div></div>
            <div class="native-form-row" v-if="engineOf(model)!=='sqlite'"><label for="connection-user">用户名：</label><div class="native-field"><input id="connection-user" class="medium-input" v-model="model.username" maxlength="256" spellcheck="false" autocomplete="off" :aria-invalid="!!errors.username"/><span v-if="errors.username" class="field-error">{{errors.username}}</span></div></div>
            <div class="native-form-row" v-if="engineOf(model)!=='sqlite'"><label for="connection-password">密码：</label><div class="native-field"><div class="native-password medium-input"><input id="connection-password" v-model="model.password" :type="visiblePassword?'text':'password'" :placeholder="model.hasPassword&&model.passwordAction==='keep'?'已保存，留空保持原密码':''" maxlength="8192" autocomplete="new-password" @input="emit('password')"/><button type="button" @click="visiblePassword=!visiblePassword" :aria-label="visiblePassword?'隐藏密码':'显示密码'"><EyeOff v-if="visiblePassword" :size="14"/><Eye v-else :size="14"/></button></div></div></div>
            <div class="native-form-row checkbox-row" v-if="engineOf(model)!=='sqlite'"><span></span><label class="native-check"><input v-model="model.rememberPassword" type="checkbox"/>保存密码与隧道凭据</label></div>
            <div v-if="model.hasPassword&&model.passwordAction==='keep'" class="native-form-row"><span></span><button type="button" class="link-button" @click="emit('forget')">清除已存凭据</button></div>
            <div v-if="engineOf(model)==='sqlite'" class="native-form-row"><label for="connection-file">数据库文件：</label><div class="native-file"><input id="connection-file" v-model="model.filePath" :aria-invalid="!!errors.filePath"/><button type="button" class="native-button browse-button" @click="chooseSQLite" aria-label="选择 SQLite 文件"><FolderOpen :size="15"/></button><span v-if="errors.filePath" class="field-error">{{errors.filePath}}</span></div></div><p v-if="pickerError" role="alert" class="field-error">{{pickerError}}</p><p class="dialog-password-note" v-if="engineOf(model)!=='sqlite'">保存的密码由本机系统凭据库保护。</p>
          </section>
          <section v-show="tab==='advanced'" role="tabpanel" id="connection-panel-advanced" aria-labelledby="connection-tabs-advanced" class="settings-panel">
            <div class="native-group"><h3>连接设置</h3><div class="native-form-row"><label for="connection-timeout">连接超时：</label><select id="connection-timeout" v-model.number="model.timeout" class="medium-input"><option :value="5">5 秒</option><option :value="10">10 秒</option><option :value="15">15 秒</option><option :value="30">30 秒</option></select></div><div class="native-form-row"><label for="connection-environment">环境分组：</label><select id="connection-environment" v-model="model.environment" class="medium-input"><option value="local">本地</option><option value="development">开发</option><option value="staging">测试</option><option value="production">生产</option></select></div></div>
            <div class="native-group"><h3>备注</h3><label class="sr-only" for="connection-notes">连接备注</label><textarea id="connection-notes" v-model="model.notes" maxlength="2000" rows="5" placeholder="记录此连接的用途或注意事项"></textarea></div>
          <div class="native-form-row"><label for="connection-group">连接组：</label><select id="connection-group" v-model="model.groupId"><option value="">未分组（我的连接）</option><option v-for="g in groups" :key="g.id" :value="g.id">{{g.name}}</option></select></div></section>
          <section v-show="tab==='databases'" role="tabpanel" id="connection-panel-databases" aria-labelledby="connection-tabs-databases" class="settings-panel"><div class="native-group"><h3>数据库设置</h3><div class="native-form-row"><label for="connection-database">{{engineOf(model)==='oracle'?'服务名：':engineOf(model)==='redis'?'逻辑库编号：':'默认数据库：'}}</label><input id="connection-database" v-model="model.database" maxlength="256" spellcheck="false"/></div><p class="settings-note">{{engineOf(model)==='oracle'?'填写 Oracle Service Name；连接后按账户可见的模式浏览。':engineOf(model)==='redis'?'默认 0；连接后显示所选逻辑库及可见的非空逻辑库。':engineOf(model)==='postgresql'?'留空使用 postgres。':engineOf(model)==='sqlserver'?'留空使用 master。':'列出当前账户有权限访问的数据库。'}}</p><div v-if="engineOf(model)==='mongodb'" class="native-form-row"><label for="mongo-authdb">认证数据库：</label><input id="mongo-authdb" v-model="model.authDatabase" placeholder="admin"/></div></div></section>
          <section v-show="tab==='ssl'" role="tabpanel" id="connection-panel-ssl" aria-labelledby="connection-tabs-ssl" class="settings-panel"><label class="native-check ssl-check"><input type="checkbox" :checked="model.tlsMode==='verify'" @change="model.tlsMode=($event.target as HTMLInputElement).checked?'verify':'disabled'"/>使用 SSL / TLS 加密</label><fieldset class="native-group ssl-fields" :disabled="model.tlsMode!=='verify'"><h3>服务器证书验证</h3><div class="native-form-row"><label for="connection-ca">CA 证书：</label><div class="native-file"><input id="connection-ca" v-model="model.caPath" placeholder="默认使用系统信任库"/><button type="button" class="native-button browse-button" @click="emit('chooseCA')" aria-label="选择 CA 证书"><FolderOpen :size="15"/></button></div></div><div class="native-form-row"><label for="connection-servername">证书主机名：</label><input id="connection-servername" v-model="model.serverName" placeholder="默认使用连接主机"/></div><p class="settings-note">验证证书的信任链与主机名。CA 证书应为 PEM 格式。</p></fieldset></section>
          <section v-show="tab==='ssh'" role="tabpanel" id="connection-panel-ssh" aria-labelledby="connection-tabs-ssh" class="settings-panel tunnel-settings">
           <label class="native-check"><input type="checkbox" :checked="model.tunnelMode==='ssh'" @change="model.tunnelMode=($event.target as HTMLInputElement).checked?'ssh':'direct'"/>使用 SSH 隧道</label>
           <fieldset :disabled="model.tunnelMode!=='ssh'" class="tunnel-fields">
            <div class="native-form-row"><label for="ssh-host">SSH 主机：</label><input id="ssh-host" v-model="model.sshHost" maxlength="253"/></div>
            <div class="native-form-row"><label for="ssh-port">SSH 端口：</label><input id="ssh-port" v-model.number="model.sshPort" type="number" min="1" max="65535" class="short-input"/></div>
            <div class="native-form-row"><label for="ssh-user">SSH 用户名：</label><input id="ssh-user" v-model="model.sshUsername" maxlength="256"/></div>
            <div class="native-form-row"><label for="ssh-auth">认证方式：</label><select id="ssh-auth" v-model="model.sshAuth"><option value="password">密码</option><option value="key">私钥</option></select></div>
            <div v-if="model.sshAuth==='password'" class="native-form-row"><label for="ssh-password">SSH 密码：</label><input id="ssh-password" type="password" v-model="model.sshPassword" autocomplete="new-password" :placeholder="model.hasPassword&&model.sshPasswordAction==='keep'?'留空沿用已存值':''" @input="model.sshPasswordAction='replace'"/></div>
            <template v-else><div class="native-form-row"><label for="ssh-key">私钥文件：</label><div class="native-file"><input id="ssh-key" v-model="model.sshKeyPath"/><button type="button" class="native-button browse-button" aria-label="选择 SSH 私钥" @click="emit('chooseSshKey')"><FolderOpen :size="15"/></button></div></div><div class="native-form-row"><label for="ssh-passphrase">私钥口令：</label><input id="ssh-passphrase" type="password" v-model="model.sshKeyPassphrase" autocomplete="new-password" :placeholder="model.hasPassword&&model.sshKeyPassphraseAction==='keep'?'留空沿用已存值':''" @input="model.sshKeyPassphraseAction='replace'"/></div></template>
            <div class="native-form-row"><label for="ssh-known-hosts">known_hosts：</label><input id="ssh-known-hosts" v-model="model.sshKnownHostsPath" placeholder="默认使用本机 .ssh/known_hosts"/></div>
            <div class="native-form-row"><label for="ssh-fingerprint">主机 SHA256 指纹：</label><input id="ssh-fingerprint" v-model="model.sshFingerprint" placeholder="可代替 known_hosts，须核对服务器指纹"/></div>
           </fieldset><p class="settings-note">常规页的主机、端口是 SSH 服务器可访问的 MySQL 地址。指纹或 known_hosts 必须验证通过。</p>
          </section>
          <section v-show="tab==='http'" role="tabpanel" id="connection-panel-http" aria-labelledby="connection-tabs-http" class="settings-panel tunnel-settings">
           <label class="native-check"><input type="checkbox" :checked="model.tunnelMode==='http'" @change="model.tunnelMode=($event.target as HTMLInputElement).checked?'http':'direct'"/>使用网站 URL / PHP 中转</label>
           <fieldset :disabled="model.tunnelMode!=='http'" class="tunnel-fields"><div class="native-form-row"><label for="http-url">脚本地址：</label><input id="http-url" v-model="model.httpUrl" placeholder="https://example.com/bajiu-mysql.php"/></div><div class="native-form-row"><label for="http-token">访问令牌：</label><input id="http-token" type="password" v-model="model.httpToken" autocomplete="new-password" :placeholder="model.hasPassword&&model.httpTokenAction==='keep'?'留空沿用已存值':''" @input="model.httpTokenAction='replace'"/></div></fieldset>
           <p class="settings-note">将发布包 tunnel/bajiu-mysql.php 部署到网站并配置访问令牌。常规页填写中转服务器可访问的 MySQL 地址。HTTPS 可保护账户、密码和令牌。</p><p class="settings-note">HTTP 与 SSH 二选一。PHP 中转按请求连接；SQL 执行与数据库管理需网站设置 BAJIU_TUNNEL_ALLOW_SQL=1，不保留跨请求事务。</p>
          </section>
        </fieldset>
        <div v-if="result||error||busy==='test'" class="dialog-feedback" :class="{failure:!!error,success:!!result}" :role="error?'alert':'status'"><LoaderCircle v-if="busy==='test'" :size="15" class="spin"/><AlertCircle v-else-if="error" :size="15"/><CheckCircle2 v-else :size="15"/><span>{{error||(busy==='test'?'正在测试连接…':'连接成功 · '+result?.serverVersion+' · '+result?.latencyMs+' ms')}}</span></div>
        <footer class="dialog-footer"><button type="button" class="native-button test-connection" @click="emit('test')" :disabled="!!busy||fatal"><LoaderCircle v-if="busy==='test'" class="spin" :size="13"/>测试连接</button><span class="dialog-footer-spacer"></span><button type="submit" class="native-button default-button" :disabled="!!busy||fatal"><LoaderCircle v-if="busy==='save'||busy==='connect'" class="spin" :size="13"/>{{connectOnSave?'连接':'确定'}}</button><button type="button" class="native-button" @click="emit('cancel')" :disabled="!!busy">取消</button></footer>
      </form>
    </section>
  </div>
</template>
