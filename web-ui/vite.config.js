import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import ElementPlus from 'unplugin-element-plus/vite'
import { resolve } from 'path'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiTarget = env.VITE_ALLBOT_API_TARGET || `http://localhost:${env.ALLBOT_WEB_PORT || '3000'}`
  const isDemo = env.VITE_ALLBOT_DEMO === 'true' || mode === 'demo'
  const basePath = env.VITE_BASE_PATH || '/'

  return {
    plugins: [
      vue(),
      Components({
        resolvers: [ElementPlusResolver()]
      }),
      ElementPlus()
    ],
    base: basePath,
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src')
      }
    },
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true
        }
      }
    },
    build: {
      outDir: isDemo ? 'dist' : '../web',
      emptyOutDir: true,
      rollupOptions: {
        output: {
          manualChunks: {
            'vue-vendor': ['vue', 'vue-router', 'pinia']
          }
        }
      }
    }
  }
})
