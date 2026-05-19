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
  if (!c.includes('kotlin-gradle-plugin')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n        classpath \"org.jetbrains.kotlin:kotlin-gradle-plugin:2.1.0\"",
    )
  }
  return c
})

// --- app/build.gradle ---
// IMPORTANT: Capacitor cap sync does NOT overwrite this file (confirmed from source)
// So our modifications persist across builds — all changes are idempotent
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

  // 3. ndk abiFilters (arm64 only)
  if (!c.includes('abiFilters') || !c.includes('arm64-v8a')) {
    c = c.replace(
      /defaultConfig\s*\{/,
      "defaultConfig {\n        ndk {\n            abiFilters 'arm64-v8a'\n        }",
    )
  }

  // 4. Version injection
  if (version) {
    const vcode = parseInt(version.replace(/\./g, '')) || 1
    c = c.replace(/versionCode\s+\d+/, `versionCode ${vcode}`)
    c = c.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`)
  }

  // 5. Signing config (only when building release)
  // Uses EXACT string anchor "minifyEnabled false" from Capacitor template
  // This is safe because cap sync never regenerates this file
  if (version && c.includes('minifyEnabled false')) {
    const scBlock =
      "\n" +
      "    signingConfigs {\n" +
      "        release {\n" +
      "            storeFile file('../../keystore/release.jks')\n" +
      "            storePassword 'encv2025'\n" +
      "            keyAlias 'encvrelease'\n" +
      "            keyPassword 'encv2025'\n" +
      "        }\n" +
      "    }\n"

    c = c.replace("android {", "android {" + scBlock)

    c = c.replace(
      "minifyEnabled false",
      "minifyEnabled true\n        shrinkResources true\n        signingConfig signingConfigs.release",
    )

    console.log('  release: signing + minify + shrink applied')
  }

  return c
})

if (version) {
  console.log(`  release mode v${version}`)
} else {
  console.log(`  debug mode`)
}
console.log('done')
