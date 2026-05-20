import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'fs'
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

// --- Root build.gradle: kotlin plugin + JitPack repository ---
patchFile(join(ANDROID_DIR, 'build.gradle'), (c) => {
  if (!c.includes('kotlin-gradle-plugin')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n        classpath \"org.jetbrains.kotlin:kotlin-gradle-plugin:2.1.0\"",
    )
  }
  if (!c.includes('jitpack.io')) {
    c = c.replace(
      /allprojects\s*\{\s*repositories\s*\{/,
      "allprojects {\n    repositories {\n        maven { url 'https://jitpack.io' }",
    )
  }
  return c
})

// --- app/build.gradle ---
const version = process.env.ENCV_VERSION || ''

patchFile(join(ANDROID_DIR, 'app', 'build.gradle'), (c) => {
  // 1. kotlin-android plugin (兼容 plugins { id } 新格式 和 apply plugin: 旧格式)
  if (!c.includes('kotlin-android') && !c.includes('org.jetbrains.kotlin.android')) {
    if (c.match(/plugins\s*\{/)) {
      // 新 DSL 格式: 在 plugins 块内添加 id 'kotlin-android'
      c = c.replace(
        /plugins\s*\{/,
        "plugins {\n    id 'kotlin-android'",
      )
      console.log('  [kotlin] injected id \'kotlin-android\' into plugins {} block')
    } else if (c.includes("apply plugin: 'com.android.application'")) {
      // 旧 apply 格式: 添加 apply plugin: 'kotlin-android'
      c = c.replace(
        "apply plugin: 'com.android.application'",
        "apply plugin: 'com.android.application'\napply plugin: 'kotlin-android'",
      )
      console.log('  [kotlin] applied kotlin-android via legacy apply')
    } else {
      console.error('  [kotlin] WARNING: could not find plugins block or com.android.application!')
    }
  } else {
    console.log('  [kotlin] kotlin-android already present')
  }

  // 2. kotlin-stdlib dependency + Logcat debug library
  if (!c.includes('kotlin-stdlib')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    implementation \"org.jetbrains.kotlin:kotlin-stdlib:2.1.0\"",
    )
  }
  if (!c.includes('Logcat')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    debugImplementation 'com.github.getActivity:Logcat:13.0'",
    )
  }

  // 3. compileOptions (required by Logcat)
  if (!c.includes('compileOptions')) {
    c = c.replace(
      /defaultConfig\s*\{/,
      "compileOptions {\n        targetCompatibility JavaVersion.VERSION_21\n        sourceCompatibility JavaVersion.VERSION_21\n    }\n\n    defaultConfig {",
    )
  }

  // 4. ndk abiFilters (arm64 only)
  if (!c.includes('abiFilters') || !c.includes('arm64-v8a')) {
    c = c.replace(
      /defaultConfig\s*\{/,
      "defaultConfig {\n        ndk {\n            abiFilters 'arm64-v8a'\n        }",
    )
  }

  // 5. Version injection
  if (version) {
    const vcode = parseInt(version.replace(/\./g, '')) || 1
    c = c.replace(/versionCode\s+\d+/, `versionCode ${vcode}`)
    c = c.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`)
  }

  // 6. Signing config (only when building release)
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

  // 7. Kotlin JVM target (Groovy DSL 兼容写法)
  // 使用 tasks.withType(KotlinCompile) 设置 jvmTarget
  // 这是 Groovy DSL 中设置 Kotlin 编译参数的标准方式
  if (!c.includes('jvmTarget') && c.includes('kotlin-android')) {
    c += `
tasks.withType(org.jetbrains.kotlin.gradle.tasks.KotlinCompile).configureEach {
    kotlinOptions {
        jvmTarget = "21"
    }
}
`
  }

  return c
})

// --- Debug-only AndroidManifest.xml: enable Logcat floating + notify entries ---
const debugManifestDir = join(ANDROID_DIR, 'app', 'src', 'debug')
const debugManifestPath = join(debugManifestDir, 'AndroidManifest.xml')
mkdirSync(debugManifestDir, { recursive: true })
const debugManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">

    <application>
        <meta-data
            android:name="LogcatWindowEntrance"
            android:value="true" />
        <meta-data
            android:name="LogcatNotifyEntrance"
            android:value="true" />
    </application>

</manifest>`
writeFileSync(debugManifestPath, debugManifest, 'utf-8')
console.log(`  created ${debugManifestPath}`)

if (version) {
  console.log(`  release mode v${version}`)
} else {
  console.log(`  debug mode`)
}
console.log('done')
