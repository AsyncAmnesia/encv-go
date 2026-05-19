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

// --- app/build.gradle ---
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

  // 3. ndk abiFilters
  if (!c.includes('arm64-v8a')) {
    c = c.replace(
      /defaultConfig\s*\{/,
      "defaultConfig {\n        ndk {\n            abiFilters 'arm64-v8a'\n        }",
    )
  }

  // 4. version injection
  if (version) {
    const vcode = parseInt(version.replace(/\./g, '')) || 1
    c = c.replace(/versionCode\s+\d+/, `versionCode ${vcode}`)
    c = c.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`)
  }

  // 5. Release signing + optimization (ONLY when version set)
  if (version && !c.includes('ENCV_RELEASE_PATCHED')) {
    // 5a. Inject signingConfigs block after "android {" — using exact anchor
    const scBlock =
      "\n" +
      "    signingConfigs {\n" +
      "        release {\n" +
      "            storeFile file('../keystore/release.jks')\n" +
      "            storePassword 'encv2025'\n" +
      "            keyAlias 'encvrelease'\n" +
      "            keyPassword 'encv2025'\n" +
      "        }\n" +
      "    }\n"
    c = c.replace("android {", "android {" + scBlock)

    // 5b. Replace release block content — use exact Capacitor-known anchors
    c = c.replace(
      "minifyEnabled false",
      "minifyEnabled true\n        shrinkResources true\n        signingConfig signingConfigs.release\n        ENCV_RELEASE_PATCHED = true",
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
