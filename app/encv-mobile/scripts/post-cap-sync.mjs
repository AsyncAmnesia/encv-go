import { readFileSync, writeFileSync, mkdirSync, existsSync, rmSync, copyFileSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ANDROID_DIR = join(__dirname, '..', 'android')
const OVERLAY_DIR = join(__dirname, '..', 'android-overlay')
const LYNX_BUNDLE_PATH = join(__dirname, '..', 'lynx-player', 'dist', 'player.lynx.bundle')

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
const rootBuildGradle = join(ANDROID_DIR, 'build.gradle')
if (!existsSync(rootBuildGradle)) {
  const stub = `buildscript {
    repositories {
        google()
        mavenCentral()
    }
    dependencies {
        classpath "org.jetbrains.kotlin:kotlin-gradle-plugin:2.1.0"
    }
}

allprojects {
    repositories {
        google()
        mavenCentral()
        maven { url 'https://jitpack.io' }
    }
}
`
  writeFileSync(rootBuildGradle, stub, 'utf-8')
  console.log(`  created ${rootBuildGradle}`)
}

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
const appBuildGradle = join(ANDROID_DIR, 'app', 'build.gradle')

if (!existsSync(appBuildGradle)) {
  const stub = `plugins {
    id 'com.android.application'
    id 'kotlin-android'
}

android {
    namespace "com.encvgo.app"
    compileSdk = 34

    defaultConfig {
        applicationId "com.encvgo.app"
        minSdk = 22
        targetSdk = 34
        versionCode = 1
        versionName = "1.0.0"
        ndk {
            abiFilters 'arm64-v8a'
        }
    }

    buildTypes {
        debug {
            debuggable true
            minifyEnabled false
        }
        release {
            minifyEnabled true
            shrinkResources true
            debuggable false
        }
    }

    compileOptions {
        targetCompatibility = JavaVersion.VERSION_21
        sourceCompatibility = JavaVersion.VERSION_21
    }
}

dependencies {
    implementation "org.jetbrains.kotlin:kotlin-stdlib:2.1.0"
    debugImplementation 'com.github.getActivity:Logcat:13.0'
}
`
  writeFileSync(appBuildGradle, stub, 'utf-8')
  console.log(`  created ${appBuildGradle}`)
}

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

  // 3. Add USE_LYNX_PLAYER build config
  if (!c.includes('USE_LYNX_PLAYER')) {
    c = c.replace(
      /defaultConfig\s*\{/,
      "defaultConfig {\n        buildConfigField \"boolean\", \"USE_LYNX_PLAYER\", \"true\"",
    )
  }

  // 3b. Enable buildConfig generation (AGP 8.0+ requires this explicitly)
  if (!c.includes('buildConfig = true') && !c.includes('buildConfig true')) {
    if (c.includes('buildFeatures')) {
      c = c.replace(
        /buildFeatures\s*\{/,
        "buildFeatures {\n        buildConfig = true",
      )
    } else {
      c = c.replace(
        /android\s*\{/,
        "android {\n    buildFeatures {\n        buildConfig = true\n    }",
      )
    }
  }

  // 3c. Add appcompat dependency for AppCompatActivity
  if (!c.includes('appcompat')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    implementation 'androidx.appcompat:appcompat:1.6.1'",
    )
  }

  // 4. Add Lynx SDK 3.7 + mpv-android-lib dependencies
  if (!c.includes('org.lynxsdk.lynx')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    implementation 'org.lynxsdk.lynx:lynx:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-jssdk:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-trace:3.7.0'\n    implementation 'org.lynxsdk.lynx:primjs:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-service-image:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-service-log:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-service-http:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-service-devtool:3.7.0'\n    implementation 'com.facebook.fresco:fresco:2.3.0'\n    implementation 'com.facebook.fresco:animated-gif:2.3.0'\n    implementation 'com.facebook.fresco:animated-webp:2.3.0'\n    implementation 'com.facebook.fresco:webpsupport:2.3.0'\n    implementation 'com.facebook.fresco:animated-base:2.3.0'\n    implementation 'com.squareup.okhttp3:okhttp:4.9.0'\n    implementation 'io.github.abdallahmehiz:mpv-android-lib:0.1.12'",
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

  // 5. jniLibs.srcDirs (ensure Gradle picks up src/main/jniLibs)
  if (!c.includes('jniLibs.srcDirs')) {
    c = c.replace(
      /android\s*\{/,
      "android {\n    sourceSets {\n        main {\n            jniLibs.srcDirs = ['src/main/jniLibs']\n        }\n    }",
    )
    console.log('  [jniLibs] added jniLibs.srcDirs')
  }

  // 6. useLegacyPackaging = true (critical: AGP 8.0+ defaults to false, which prevents .so extraction)
  if (!c.includes('useLegacyPackaging')) {
    c = c.replace(
      /android\s*\{/,
      "android {\n    packaging {\n        jniLibs {\n            useLegacyPackaging = true\n        }\n    }",
    )
    console.log('  [packaging] added useLegacyPackaging = true')
  }

  // 7. Version injection
  if (version) {
    const vcode = parseInt(version.replace(/\./g, '')) || 1
    c = c.replace(/versionCode\s+\d+/, `versionCode ${vcode}`)
    c = c.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`)
  }

  // 8. Signing config (only when building release)
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

  // 9. Kotlin JVM target (Groovy DSL 兼容写法)
  if (!c.includes('jvmTarget') && c.includes('kotlin-android')) {
    c += `
tasks.withType(org.jetbrains.kotlin.gradle.tasks.KotlinCompile).configureEach {
    kotlinOptions {
        jvmTarget = "21"
    }
}
`
  }

  // 10. 显式声明 sourceSets（确保 Kotlin 编译器能找到 app/src/main/java 下的 .kt 文件）
  if (!c.includes('sourceSets') && c.includes('kotlin-android')) {
    c += `
android.sourceSets {
    main.java.srcDirs += 'src/main/java'
}
`
    console.log('  [kotlin] added explicit sourceSets for kotlin sources')
  }

  // 11. 打印 build.gradle 关键配置用于诊断（仅 debug 模式）
  if (!version) {
    const lines = c.split('\n')
    console.log('  [diag] --- build.gradle key lines ---')
    lines.forEach((line, i) => {
      if (line.match(/kotlin|namespace|sourceSets|apply plugin|plugins\s*\{|jniLibs|useLegacyPackaging/)) {
        console.log(`  [diag] L${i + 1}: ${line.trim()}`)
      }
    })
    console.log(`  [diag] total: ${lines.length} lines`)
  }

  return c
})

// --- Overlay files: MainActivity.kt, GoProcessPlugin.kt, EncvGoService.kt, PlayerActivity*.kt, manifest, proguard, network config ---
const JAVA_DIR = join(ANDROID_DIR, 'app', 'src', 'main', 'java', 'com', 'encvgo', 'app')

if (existsSync(JAVA_DIR)) {
  rmSync(JAVA_DIR, { recursive: true, force: true })
}
mkdirSync(JAVA_DIR, { recursive: true })

for (const f of ['MainActivity.kt', 'GoProcessPlugin.kt', 'EncvGoService.kt', 'PlayerActivity.kt', 'PlayerActivityLynx.kt', 'PlayerActivityCapacitor.kt', 'MpvPlayerModule.kt', 'GoBackendModule.kt', 'EncvApplication.kt', 'PlayerTemplateProvider.kt']) {
  const src = join(OVERLAY_DIR, 'app', 'src', 'main', 'java', 'com', 'encvgo', 'app', f)
  if (existsSync(src)) {
    copyFileSync(src, join(JAVA_DIR, f))
    console.log(`  overlay: ${f}`)
  } else {
    console.error(`  overlay: missing ${src}`)
  }
}

// Copy layout file
const RES_LAYOUT_DIR = join(ANDROID_DIR, 'app', 'src', 'main', 'res', 'layout')
mkdirSync(RES_LAYOUT_DIR, { recursive: true })
const layoutSrc = join(OVERLAY_DIR, 'app', 'src', 'main', 'res', 'layout', 'lynx_player_activity.xml')
if (existsSync(layoutSrc)) {
  copyFileSync(layoutSrc, join(RES_LAYOUT_DIR, 'lynx_player_activity.xml'))
  console.log('  overlay: layout/lynx_player_activity.xml')
}

// Copy lynx bundle to assets if exists
if (existsSync(LYNX_BUNDLE_PATH)) {
  const assetsDir = join(ANDROID_DIR, 'app', 'src', 'main', 'assets')
  mkdirSync(assetsDir, { recursive: true })
  copyFileSync(LYNX_BUNDLE_PATH, join(assetsDir, 'player.lynx.bundle'))
  console.log('  bundle: copied player.lynx.bundle to assets')
}

const jniLibsDir = join(ANDROID_DIR, 'app', 'src', 'main', 'jniLibs', 'arm64-v8a')
mkdirSync(jniLibsDir, { recursive: true })
console.log('  ensured jniLibs/arm64-v8a directory exists')

const overlayManifestSrc = join(OVERLAY_DIR, 'app', 'src', 'main', 'AndroidManifest.xml')
const appManifestDest = join(ANDROID_DIR, 'app', 'src', 'main', 'AndroidManifest.xml')
if (existsSync(overlayManifestSrc)) {
  copyFileSync(overlayManifestSrc, appManifestDest)
  console.log('  overlay: AndroidManifest.xml')
}

const proguardSrc = join(OVERLAY_DIR, 'proguard-rules.pro')
if (existsSync(proguardSrc)) {
  copyFileSync(proguardSrc, join(ANDROID_DIR, 'app', 'proguard-rules.pro'))
  console.log('  overlay: proguard-rules.pro')
}

const xmlDir = join(ANDROID_DIR, 'app', 'src', 'main', 'res', 'xml')
mkdirSync(xmlDir, { recursive: true })
const xmlSrc = join(OVERLAY_DIR, 'app', 'src', 'main', 'res', 'xml', 'network_security_config.xml')
if (existsSync(xmlSrc)) {
  copyFileSync(xmlSrc, join(xmlDir, 'network_security_config.xml'))
  console.log('  overlay: network_security_config.xml')
}

const mainActivityPath = join(JAVA_DIR, 'MainActivity.kt')
if (existsSync(mainActivityPath)) {
  const content = readFileSync(mainActivityPath, 'utf-8')
  const count = (content.match(/class MainActivity/g) || []).length
  if (count !== 1) {
    console.error(`  ERROR: Found ${count} MainActivity class declarations (expected 1)`)
    process.exit(1)
  }
  console.log(`  verified: 1 MainActivity class declaration ✓`)
}

// --- Copy config.mobile.json to Android assets directory ---
const androidAssetsDir = join(ANDROID_DIR, 'app', 'src', 'main', 'assets')
mkdirSync(androidAssetsDir, { recursive: true })
const configSrc = join(__dirname, '..', 'assets', 'config.mobile.json')
if (existsSync(configSrc)) {
  copyFileSync(configSrc, join(androidAssetsDir, 'config.mobile.json'))
  console.log('  overlay: config.mobile.json → Android assets')
} else {
  console.error('  WARNING: assets/config.mobile.json not found')
}

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

// --- 包名一致性验证 ---
for (const f of ['MainActivity.kt', 'GoProcessPlugin.kt', 'EncvGoService.kt', 'PlayerActivity.kt']) {
  const fp = join(JAVA_DIR, f)
  if (existsSync(fp)) {
    const src = readFileSync(fp, 'utf-8')
    const pkg = (src.match(/^package\s+(\S+)/m) || [])[1]
    if (pkg !== 'com.encvgo.app') {
      console.error(`  ERROR: ${f} package="${pkg}" ≠ expected "com.encvgo.app"`)
      process.exit(1)
    }
    console.log(`  pkg-ok: ${f} → ${pkg}`)
  }
}

console.log('done')
