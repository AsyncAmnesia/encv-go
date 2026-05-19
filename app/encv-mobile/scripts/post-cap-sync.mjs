import { readFileSync, writeFileSync, existsSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ANDROID_DIR = join(__dirname, '..', 'android')
const APP_GRADLE = join(ANDROID_DIR, 'app', 'build.gradle')

console.log('encv-post-cap-sync: applying Android customizations...')

// --- Root build.gradle: add kotlin plugin ---
patchFile(join(ANDROID_DIR, 'build.gradle'), (c) => {
  if (c.includes('kotlin-gradle-plugin')) return c
  return c.replace(
    'dependencies {',
    "dependencies {\n        classpath \"org.jetbrains.kotlin:kotlin-gradle-plugin:2.1.0\"",
    1,
  )
})

// --- app/build.gradle: MINIMAL changes only ---
const version = process.env.ENCV_VERSION || ''

patchFile(APP_GRADLE, (c) => {
  // Add kotlin-android plugin
  if (!c.includes("'kotlin-android'")) {
    c = c.replace(
      "'com.android.application'",
      "'com.android.application'\n    'kotlin-android'",
      1,
    )
  }

  // Add kotlin-stdlib dependency
  if (!c.includes('kotlin-stdlib')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    implementation \"org.jetbrains.kotlin:kotlin-stdlib:2.1.0\"",
      1,
    )
  }

  // Apply our release config file (idempotent - checks for existing apply)
  const applyLine = "apply from: '../encv-release.gradle'"
  if (!c.includes('encv-release.gradle')) {
    // Insert after the plugins block (first closing })
    c = c.replace(/(plugins\s*\{[^}]*\})/, '$1\n\n' + applyLine)
  }

  // Set version in defaultConfig (only when ENCV_VERSION is set)
  if (version) {
    const vcode = parseInt(version.replace(/\./g, '')) || 1
    // Replace existing versionCode/Name or inject into defaultConfig
    if (c.includes('versionCode') && !c.includes(`versionCode ${vcode}`)) {
      c = c.replace(/versionCode\s+\d+/, `versionCode ${vcode}`)
      c = c.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`)
    } else if (c.includes('defaultConfig')) {
      c = c.replace(
        /(defaultConfig\s*\{[^}]*?)\}/,
        `$1\n        versionName "${version}"\n        versionCode ${vcode}\n    }`,
      )
    }
  }

  return c
})

if (version) {
  console.log(`  release mode v${version}: encv-release.gradle applied`)
} else {
  console.log(`  debug mode: no release optimization`)
}

console.log('encv-post-cap-sync: done')

function patchFile(filePath, transformer) {
  const content = readFileSync(filePath, 'utf-8')
  const modified = transformer(content)
  if (modified !== content) {
    writeFileSync(filePath, modified, 'utf-8')
    console.log(`  patched ${filePath}`)
  }
}
