<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import { Folder, LoaderCircle } from '@lucide/vue'

const props=defineProps<{name:string;busy:boolean;error:string}>()
const emit=defineEmits<{'update:name':[value:string];save:[];cancel:[]}>()
const input=ref<HTMLInputElement|null>(null)
onMounted(()=>nextTick(()=>{input.value?.focus();input.value?.select()}))
watch(()=>[props.error,props.busy],()=>{if(props.error&&!props.busy)nextTick(()=>input.value?.focus())})
function blur(){if(props.busy)return;if(props.name.trim())emit('save');else emit('cancel')}
</script>

<template>
 <div class="inline-group-edit" role="treeitem" aria-label="编辑组名">
  <form class="tree-row tree-group-editor" @submit.prevent="emit('save')" @click.stop @keydown.stop>
   <span class="tree-expander" aria-hidden="true"></span><Folder :size="16" class="folder-icon"/>
   <input ref="input" class="inline-group-name" aria-label="组名" :value="name" @input="emit('update:name',($event.target as HTMLInputElement).value)" maxlength="80" :disabled="busy" :aria-invalid="!!error" :aria-describedby="error?'inline-group-error':undefined" @keydown.enter.prevent.stop="emit('save')" @keydown.esc.prevent.stop="emit('cancel')" @blur="blur"/>
   <span v-if="busy" role="status"><LoaderCircle :size="12" class="spin"/><span class="sr-only">正在保存分组</span></span>
  </form>
  <p v-if="error" id="inline-group-error" class="inline-group-error" role="alert">{{error}}</p>
 </div>
</template>
