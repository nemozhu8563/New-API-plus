import path from 'path'
import { createRequire } from 'module'
import { fileURLToPath } from 'url'
import { defineConfig, loadEnv } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const workspaceRequire = createRequire(path.resolve(__dirname, '../package.json'))
const workspaceNodeModules = path.resolve(__dirname, '../node_modules')
const resolveWorkspacePackage = (packageName: string) => {
  const packageDir = path.resolve(workspaceNodeModules, packageName)
  workspaceRequire.resolve(packageName)
  return packageDir
}
const semiUiDir = resolveWorkspacePackage('@douyinfe/semi-ui')
const semiIconsDir = resolveWorkspacePackage('@douyinfe/semi-icons')
const semiIllustrationsDir = resolveWorkspacePackage('@douyinfe/semi-illustrations')
const lobeIconsDir = resolveWorkspacePackage('@lobehub/icons')
const reactDir = resolveWorkspacePackage('react')
const reactDomDir = resolveWorkspacePackage('react-dom')
const semiDateFnsDir = path.resolve(
  resolveWorkspacePackage('@douyinfe/semi-foundation'),
  '../semi-foundation/node_modules/date-fns',
)

export default defineConfig(({ envMode }) => {
  const env = loadEnv({ mode: envMode, prefixes: ['VITE_'] })
  const clientServerUrl =
    process.env.VITE_REACT_APP_SERVER_URL ||
    env.rawPublicVars.VITE_REACT_APP_SERVER_URL ||
    ''
  const proxyServerUrl =
    clientServerUrl ||
    'http://localhost:3000'
  const isProd = envMode === 'production'
  const devProxy = Object.fromEntries(
    (['/api', '/mj', '/pg'] as const).map((key) => [
      key,
      { target: proxyServerUrl, changeOrigin: true },
    ]),
  ) as Record<string, { target: string; changeOrigin: boolean }>

  return {
    plugins: [pluginReact()],
    source: {
      entry: {
        index: './src/index.jsx',
      },
      define: {
        'import.meta.env.VITE_REACT_APP_SERVER_URL': JSON.stringify(
          clientServerUrl,
        ),
      },
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        '@douyinfe/semi-icons': semiIconsDir,
        '@douyinfe/semi-illustrations': semiIllustrationsDir,
        '@douyinfe/semi-ui': semiUiDir,
        '@douyinfe/semi-ui/dist/css/semi.css': path.resolve(
          semiUiDir,
          'dist/css/semi.css',
        ),
        '@lobehub/icons': lobeIconsDir,
        'date-fns': semiDateFnsDir,
        'react': reactDir,
        'react-dom': reactDomDir,
      },
    },
    html: {
      template: './index.html',
    },
    server: {
      host: '0.0.0.0',
      strictPort: false,
      proxy: devProxy,
    },
    output: {
      minify: isProd,
      target: 'web',
      distPath: {
        root: 'dist',
      },
    },
    performance: {
      removeConsole: isProd ? ['log'] : false,
      buildCache: {
        cacheDigest: [process.env.VITE_REACT_APP_VERSION],
      },
    },
    tools: {
      rspack: {
        module: {
          rules: [
            {
              test: /src[\\/].*\.js$/,
              type: 'javascript/auto',
              use: [
                {
                  loader: 'builtin:swc-loader',
                  options: {
                    jsc: {
                      parser: {
                        syntax: 'ecmascript',
                        jsx: true,
                      },
                      transform: {
                        react: {
                          runtime: 'automatic',
                          development: !isProd,
                          refresh: !isProd,
                        },
                      },
                    },
                  },
                },
              ],
            },
          ],
        },
      },
    },
  }
})
