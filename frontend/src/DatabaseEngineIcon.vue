<script setup lang="ts">
import {computed} from 'vue'
import {databaseEngines,engineLabel} from './databaseEngines'
const props=withDefaults(defineProps<{engine?:string;size?:number;state?:'connected'|'disconnected';labelled?:boolean}>(),{size:16,labelled:false})
const kind=computed(()=>{const value=(props.engine||'mysql').toLowerCase();return databaseEngines.some(e=>e.value===value)?value:'unknown'})
const name=computed(()=>kind.value==='unknown'?'数据库':engineLabel({engine:kind.value}))
const description=computed(()=>name.value+(props.state==='connected'?' · 已连接':props.state==='disconnected'?' · 未连接':''))
</script>
<template>
 <svg class="database-engine-icon" :class="state?'is-'+state:undefined" :data-database-engine="kind" :width="size" :height="size" viewBox="0 0 32 32" fill="none" stroke-linecap="round" stroke-linejoin="round" :role="labelled?'img':undefined" :aria-label="labelled?description:undefined" :aria-hidden="labelled?undefined:true" focusable="false">
  <title v-if="labelled">{{description}}</title>
  <!-- Original, compact silhouettes share a 32-unit optical grid. No font or remote asset is required. -->
  <g v-if="kind==='mysql'" stroke="#00758f" stroke-width="1.9">
   <path fill="#e4f4f8" d="M4 23c3-6 8-11 14-12 1-4 4-7 7-7l-1 6 5 2-6 2c-1 4-5 7-9 8l-3 6-2-5-5 1 2-4"/>
   <path d="M10 19c4-4 7-5 11-6m-8 5 4 3m-8 2-4 5"/>
   <circle cx="23.3" cy="10.1" r=".9" fill="#00758f" stroke="none"/>
  </g>
  <g v-else-if="kind==='postgresql'" stroke="#244e76" stroke-width="1.5">
   <path fill="#3e78ad" d="M6 6C2 9 4 21 8 23l3-5 2 5 4-1 3 3 5-3c4-3 5-12 1-16-5-4-15-4-20 0Z"/>
   <path fill="#4f8cbd" d="M12 8c2-2 6-2 8 0l1 10-1 7c0 4-4 5-7 3l1-3 2 1 1-6-4-3Z"/>
   <path stroke="#fff" d="m7 10 2 7m16-7-2 8m-9-8 2 1m2-1 1 1m-7 10-3 3m10-1 3 2"/>
  </g>
  <g v-else-if="kind==='sqlserver'" stroke-width="1.1">
   <path fill="#c73842" d="M6 3h18L12 13l15 5-14 12H5l13-11L4 14 17 6H6Z"/>
   <path stroke="#fff" d="m10 4 9 6m-4-5 6 3M8 11l12 4M6 14l16 4m-12 3 10 3m-14 3 10 1M12 8l6 18m-1-17 5 13"/>
  </g>
  <g v-else-if="kind==='oracle'">
   <rect x="2.5" y="8" width="27" height="16" rx="8" stroke="#e52b3e" stroke-width="4.3"/>
  </g>
  <g v-else-if="kind==='sqlite'">
   <path fill="#1c658d" d="M5 5h18v24H5Z"/>
   <path fill="#8ac9e5" d="M22 3c-9 2-15 10-13 19 8-1 14-8 17-18Z"/>
   <path stroke="#fff" stroke-width="1.6" d="M9 27c2-8 7-15 14-21m-9 11 5-1m-2-4 1-4"/>
  </g>
  <g v-else-if="kind==='mariadb'" stroke="#8c6847" stroke-width="1.7">
   <path fill="#eee0ce" d="M3 23c4-1 5-6 9-8l7-1c2-5 5-7 10-7l-1 7-5 2c-1 6-7 10-14 9l-6 3 1-4-2-1Z"/>
   <path d="m13 20 4 3m7-9 4-1m-20 10 3-3"/>
   <circle cx="26" cy="10.5" r=".85" fill="#5c4534" stroke="none"/>
  </g>
  <g v-else-if="kind==='mongodb'">
   <path fill="#47a248" d="M16 2C6 10 5 19 15 27l1 4 1-4C27 19 26 10 16 2Z"/>
   <path fill="#2d7b3d" d="M16 3v24c9-7 9-15 0-24Z"/>
   <path stroke="#b8d699" stroke-width="1.1" d="M16 7v20"/>
   <path stroke="#795d42" stroke-width="1.7" d="m16 26 .2 5"/>
  </g>
  <g v-else-if="kind==='redis'" stroke="#fff" stroke-width="1.1">
   <path fill="#a92b32" d="m3 20 13-6 13 6v4l-13 6-13-6Z"/>
   <path fill="#c83c43" d="m3 14 13-6 13 6v4l-13 6-13-6Z"/>
   <path fill="#e25459" d="m3 8 13-6 13 6v4l-13 6-13-6Z"/>
   <path d="m3 8 13 6 13-6"/>
   <path fill="#fff" stroke="none" d="m15 5 5 3-5 3-5-3Zm7 5 3 1-3 2-3-2Z"/>
  </g>
  <g v-else stroke="#697e8c" stroke-width="2">
   <ellipse cx="16" cy="7" rx="11" ry="4"/>
   <path d="M5 7v18c0 5 22 5 22 0V7M5 16c0 5 22 5 22 0"/>
  </g>
  <circle v-if="state" class="database-engine-status" cx="26.5" cy="26.5" r="4.2" :fill="state==='connected'?'#199454':'#8c969f'" stroke="#fff" stroke-width="1.8"/>
 </svg>
</template>
