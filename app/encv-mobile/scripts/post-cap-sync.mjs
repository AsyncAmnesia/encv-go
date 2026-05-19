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

// --- Root build.gradle: add kotlin plugin ---
patchFile(join(ANDROID_DIR, 'build.gradle'), (c) => {
  if (c.includes('kotlin-gradle-plugin')) return c
  return c.replace(
    'dependencies {',
    "dependencies {\n        classpath \"org.jetbrains.kotlin:kotlin-gradle-plugin:2.1.0\"",
    1,
  )
})

// --- app/build.gradle: all modifications ---
const version = process.env.ENCV_VERSION || ''

patchFile(join(ANDROID_DIR, 'app', 'build.gradle'), (c) => {
  if (!c.includes("'kotlin-android'")) {
    c = c.replace(
      "'com.android.application'",
      "'com.android.application'\n    'kotlin-android'",
      1,
    )
  }

  if (!c.includes('kotlin-stdlib')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    implementation \"org.jetbrains.kotlin:kotlin-stdlib:2.1.0\"",
      1,
    )
  }

  if (!c.includes('abiFilters') || !c.includes('arm64-v8a')) {
    c = c.replace(
      /(defaultConfig\s*\{)/,
      "$1\n    ndk {\n        abiFilters 'arm64-v8a'\n    }",
    )
  }

  if (version) {
    const vcode = parseInt(version.replace(/\./g, '')) || 1
    c = c.replace(
      /(defaultConfig\s*\{[^}]*?)\}/,
      `$1\n        versionName "${version}"\n        versionCode ${vcode}\n    }`,
    )
  }

  if (version) {
    const scBlock =
      'android {\n' +
      '    signingConfigs {\n' +
      '        release {\n' +
      "            storeFile file('../keystore/release.jks')\n" +
      "            storePassword 'encv2025'\n" +
      "            keyAlias 'encvrelease'\n" +
      "            keyPassword 'encv2025'\n" +
      '        }\n' +
      '    }\n'
    c = c.replace(/android\s*\{/, scBlock)

    c = c.replace(
      /release\s*\{[^}]*(?:\{[^}]*\}[^}]*)*\}/,
      `release {
                        minifyEnabled true
                        shrinkResources true
                        signingConfig signingConfigs.release
                    }`,
    )
  }

  return c
})

if (version) {
  console.log(`  release mode v${version}: signing + minify + shrink enabled`)
} else {
  console.log(`  debug mode: no signing optimization`)
}

console.log('encv-post-cap-sync: done')
