import { readFileSync } from 'fs'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const localesDir = resolve(__dirname, '..', 'src', 'locales')

function parseKeys(filePath) {
  const content = readFileSync(filePath, 'utf-8')
  const keys = new Set()
  const regex = /^\s+'([^']+)':/gm
  let match
  while ((match = regex.exec(content)) !== null) {
    keys.add(match[1])
  }
  return keys
}

const enKeys = parseKeys(resolve(localesDir, 'en-US.ts'))
const zhKeys = parseKeys(resolve(localesDir, 'zh-CN.ts'))

let exitCode = 0

for (const key of enKeys) {
  if (!zhKeys.has(key)) {
    console.error(`❌ zh-CN.ts 缺少 key: "${key}"`)
    exitCode = 1
  }
}

for (const key of zhKeys) {
  if (!enKeys.has(key)) {
    console.error(`❌ en-US.ts 缺少 key: "${key}"`)
    exitCode = 1
  }
}

if (exitCode === 0) {
  console.log(`✅ 双语言 key 一致 (${enKeys.size} 个)`)
} else {
  console.error(`\n差异: en-US.ts 有 ${enKeys.size} 个 key, zh-CN.ts 有 ${zhKeys.size} 个 key`)
}

process.exit(exitCode)
