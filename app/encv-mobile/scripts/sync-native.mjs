import { rmSync, mkdirSync, copyFileSync, cpSync, readdirSync, statSync, existsSync, readFileSync, writeFileSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ANDROID_DIR = join(__dirname, '..', 'android')
const OVERLAY_DIR = join(__dirname, '..', 'android-overlay')
const LYNX_BUNDLE_PATH = join(__dirname, '..', 'lynx-player', 'dist', 'player.lynx.bundle')
const JAVA_DIR = join(ANDROID_DIR, 'app', 'src', 'main', 'java', 'com', 'encvgo', 'app')

console.log('encv-sync-native: syncing Kotlin + native files to Android project...')

if (!existsSync(JAVA_DIR)) {
  mkdirSync(JAVA_DIR, { recursive: true })
}

const ktFiles = []

function syncKtFiles(srcDir, destDir, relBase = '') {
  if (!existsSync(srcDir)) {
    console.warn(`  kt: source dir not found: ${srcDir}`)
    return
  }
  function walk(dir, rel) {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry)
      const relPath = rel ? join(rel, entry) : entry
      if (statSync(full).isDirectory()) {
        walk(full, relPath)
      } else if (entry.endsWith('.kt')) {
        const dest = join(destDir, relPath)
        mkdirSync(dirname(dest), { recursive: true })
        copyFileSync(full, dest)
        ktFiles.push(relPath)
        console.log(`  kt: ${relPath}`)
      }
    }
  }
  walk(srcDir, relBase)
}

syncKtFiles(
  join(OVERLAY_DIR, 'app', 'src', 'main', 'java', 'com', 'encvgo', 'app'),
  JAVA_DIR,
  ''
)
syncKtFiles(
  join(OVERLAY_DIR, 'app', 'src', 'main', 'java', 'is'),
  join(ANDROID_DIR, 'app', 'src', 'main', 'java', 'is'),
  ''
)

let javaCleaned = 0
function cleanConflictingJava(ktDir) {
  if (!existsSync(ktDir)) return
  for (const entry of readdirSync(ktDir)) {
    const full = join(ktDir, entry)
    if (statSync(full).isDirectory()) {
      cleanConflictingJava(full)
    } else if (entry.endsWith('.kt')) {
      const javaFile = join(ktDir, entry.replace(/\.kt$/, '.java'))
      if (existsSync(javaFile)) {
        rmSync(javaFile)
        javaCleaned++
        console.log(`  clean: removed conflicting ${entry.replace(/\.kt$/, '.java')}`)
      }
    }
  }
}
cleanConflictingJava(JAVA_DIR)
if (javaCleaned > 0) console.log(`  clean: ${javaCleaned} conflicting .java files removed`)

console.log(`  kt: ${ktFiles.length} files synced`)

const overlayJni = join(OVERLAY_DIR, 'app', 'src', 'main', 'jniLibs')
const targetJni = join(ANDROID_DIR, 'app', 'src', 'main', 'jniLibs')
const ALLOWED_ABIS = ['arm64-v8a']
if (existsSync(overlayJni)) {
  for (const abi of readdirSync(overlayJni)) {
    if (!ALLOWED_ABIS.includes(abi)) continue
    const abiDir = join(overlayJni, abi)
    if (statSync(abiDir).isDirectory()) {
      const targetAbi = join(targetJni, abi)
      mkdirSync(targetAbi, { recursive: true })
      for (const so of readdirSync(abiDir)) {
        if (so.endsWith('.so')) {
          copyFileSync(join(abiDir, so), join(targetAbi, so))
          console.log(`  so: ${abi}/${so}`)
        }
      }
    }
  }
} else {
  mkdirSync(join(targetJni, 'arm64-v8a'), { recursive: true })
  console.log('  so: ensured jniLibs/arm64-v8a (no overlay)')
}

const overlayJniSrc = join(OVERLAY_DIR, 'app', 'src', 'main', 'jni')
const targetJniSrc = join(ANDROID_DIR, 'app', 'src', 'main', 'jni')
if (existsSync(overlayJniSrc)) {
  if (existsSync(targetJniSrc)) rmSync(targetJniSrc, { recursive: true })
  cpSync(overlayJniSrc, targetJniSrc, { recursive: true })
  console.log('  jni: synced')
}

const overlayInc = join(OVERLAY_DIR, 'app', 'src', 'main', 'include')
const targetInc = join(ANDROID_DIR, 'app', 'src', 'main', 'include')
if (existsSync(overlayInc)) {
  if (existsSync(targetInc)) rmSync(targetInc, { recursive: true })
  cpSync(overlayInc, targetInc, { recursive: true })
  console.log('  include: synced')
}

const overlayManifest = join(OVERLAY_DIR, 'app', 'src', 'main', 'AndroidManifest.xml')
const targetManifest = join(ANDROID_DIR, 'app', 'src', 'main', 'AndroidManifest.xml')
if (existsSync(overlayManifest)) {
  copyFileSync(overlayManifest, targetManifest)
  console.log('  AndroidManifest.xml: synced from overlay')
}

const overlayProguard = join(OVERLAY_DIR, 'proguard-rules.pro')
const targetProguard = join(ANDROID_DIR, 'app', 'proguard-rules.pro')
if (existsSync(overlayProguard)) {
  copyFileSync(overlayProguard, targetProguard)
  console.log('  proguard-rules.pro: synced from overlay')
}

const overlayLayout = join(OVERLAY_DIR, 'app', 'src', 'main', 'res', 'layout')
const targetLayout = join(ANDROID_DIR, 'app', 'src', 'main', 'res', 'layout')
if (existsSync(overlayLayout)) {
  mkdirSync(targetLayout, { recursive: true })
  cpSync(overlayLayout, targetLayout, { recursive: true })
  console.log('  layout: synced from overlay')
}

const overlayXml = join(OVERLAY_DIR, 'app', 'src', 'main', 'res', 'xml')
const targetXml = join(ANDROID_DIR, 'app', 'src', 'main', 'res', 'xml')
if (existsSync(overlayXml)) {
  mkdirSync(targetXml, { recursive: true })
  cpSync(overlayXml, targetXml, { recursive: true })
  console.log('  res/xml: synced from overlay')
}

const overlayGradleProps = join(OVERLAY_DIR, 'gradle.properties')
const targetGradleProps = join(ANDROID_DIR, 'gradle.properties')
if (existsSync(overlayGradleProps)) {
  copyFileSync(overlayGradleProps, targetGradleProps)
  console.log('  gradle.properties: synced from overlay')
}

const assetsDir = join(ANDROID_DIR, 'app', 'src', 'main', 'assets')
mkdirSync(assetsDir, { recursive: true })
const configMobile = join(__dirname, '..', 'assets', 'config.mobile.json')
if (existsSync(configMobile)) {
  copyFileSync(configMobile, join(assetsDir, 'config.mobile.json'))
  console.log('  config.mobile.json: synced')
}

const mainActivity = join(JAVA_DIR, 'MainActivity.kt')
if (existsSync(mainActivity)) {
  const content = readFileSync(mainActivity, 'utf-8')
  const count = (content.match(/class MainActivity/g) || []).length
  if (count !== 1) {
    console.error(`  ERROR: ${count} MainActivity declarations (expected 1)`)
    process.exit(1)
  }
  console.log('  MainActivity: 1 declaration ✓')
}

if (!existsSync(LYNX_BUNDLE_PATH)) {
  console.error('ERROR: Lynx bundle not found.')
  console.error('Run: cd lynx-player && npm install && npm run build')
  process.exit(1)
}
console.log('  Lynx bundle: exists ✓')

// --- Gradle pre-injection (ensure Trapeze target nodes exist) ---
console.log('encv-sync-native: pre-injecting Gradle nodes for Trapeze...')

function patchFile(filePath, transformer) {
  if (!existsSync(filePath)) {
    console.warn(`  gradle: file not found: ${filePath}`)
    return
  }
  const content = readFileSync(filePath, 'utf-8')
  const modified = transformer(content)
  if (modified !== content) {
    writeFileSync(filePath, modified, 'utf-8')
    console.log(`  gradle: pre-injected ${filePath}`)
  }
}

patchFile(join(ANDROID_DIR, 'build.gradle'), (c) => {
  if (!c.includes('kotlin-gradle-plugin')) {
    c = c.replace(
      /dependencies\s*\{/,
      "dependencies {\n        classpath \"org.jetbrains.kotlin:kotlin-gradle-plugin:2.1.0\""
    )
  }
  if (!c.includes('jitpack.io')) {
    c = c.replace(
      /allprojects\s*\{\s*repositories\s*\{/,
      "allprojects {\n    repositories {\n        maven { url 'https://jitpack.io' }"
    )
  }
  return c
})

patchFile(join(ANDROID_DIR, 'app', 'build.gradle'), (c) => {
  if (!c.includes('plugins {') && !c.includes('plugins{')) {
    c = "plugins {\n    id 'kotlin-android'\n}\n\n" + c
  }
  return c
})

console.log('encv-sync-native: done')
