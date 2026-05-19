import { readFileSync, writeFileSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ANDROID_DIR = join(__dirname, '..', 'android')

function patchFile(filePath, transformer) {
  const content = readFileSync(filePath, 'utf-8')
  const modified = transformer(content)
  if (modified !== content) {
    writeFileSync(filePath, modified, 'utf-8')
    console.log(`  patched ${filePath}`)
  }
}

console.log('encv-post-cap-sync: applying Android customizations...')

// --- Root build.gradle: kotlin plugin ---
patchFile(join(ANDROID_DIR, 'build.gradle'), (c) => {
  if (c.includes('kotlin-gradle-plugin')) return c
  return c.replace(
    'dependencies {',
    "dependencies {\n        classpath \"org.jetbrains.kotlin:kotlin-gradle-plugin:2.1.0\"",
  )
})

// --- app/build.gradle: MINIMAL changes ---
const version = process.env.ENCV_VERSION || ''

patchFile(join(ANDROID_DIR, 'app', 'build.gradle'), (c) => {
  // 1. kotlin-android plugin
  if (!c.includes("'kotlin-android'")) {
    c = c.replace(
      "'com.android.application'",
      "'com.android.application'\n    'kotlin-android'",
    )
  }

  // 2. kotlin-stdlib dependency
  if (!c.includes('kotlin-stdlib')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    implementation \"org.jetbrains.kotlin:kotlin-stdlib:2.1.0\"",
    )
  }

  // 3. Apply our release config (idempotent — cap sync does NOT overwrite this file)
  const applyLine = "apply from: '../encv-release.gradle'"
  if (!c.includes('encv-release.gradle')) {
    c = c.replace(/(plugins\s*\{[^}]*\})/, '$1\n\n' + applyLine)
  }

  // 4. Version injection
  if (version) {
    const vcode = parseInt(version.replace(/\./g, '')) || 1
    c = c.replace(/versionCode\s+\d+/, `versionCode ${vcode}`)
    c = c.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`)
  }

  return c
})

if (version) {
  console.log(`  release mode v${version}`)
} else {
  console.log(`  debug mode`)
}
console.log('done')
