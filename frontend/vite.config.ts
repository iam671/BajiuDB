import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
export default defineConfig({ plugins:[vue()], clearScreen:false, server:{strictPort:true}, test:{ environment:'jsdom', include:['src/**/*.test.ts'] } })
