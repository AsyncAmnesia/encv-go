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
  // 1. kotlin-android plugin
  if (!c.includes("'kotlin-android'")) {
    c = c.replace(
      "'com.android.application'",
      "'com.android.application'\n    'kotlin-android'",
    )
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

  // 2b. compileOptions (required by Logcat)
  if (!c.includes('compileOptions')) {
    c = c.replace(
      /defaultConfig\s*\{/,
      "compileOptions {\n        targetCompatibility JavaVersion.VERSION_17\n        sourceCompatibility JavaVersion.VERSION_17\n    }\n\n    defaultConfig {",
    )
  }

  // 2c. kotlinOptions (required for Kotlin compilation)
  if (!c.includes('kotlinOptions')) {
    c = c.replace(
      /compileOptions\s*\{[^}]*\}/,
      "compileOptions {\n        targetCompatibility JavaVersion.VERSION_17\n        sourceCompatibility JavaVersion.VERSION_17\n    }\n\n    kotlinOptions {\n        jvmTarget = '17'\n    }",
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

// --- capacitor.plugins.json: register local GoProcessPlugin ---
const assetsDir = join(ANDROID_DIR, 'app', 'src', 'main', 'assets')
const pluginsJsonPath = join(assetsDir, 'capacitor.plugins.json')
const goProcessEntry = JSON.stringify({
  pkg: 'encv-mobile',
  classpath: 'com.encvgo.app.GoProcessPlugin'
})

if (existsSync(pluginsJsonPath)) {
  let pluginsJson = readFileSync(pluginsJsonPath, 'utf-8')
  if (!pluginsJson.includes('GoProcessPlugin')) {
    try {
      const arr = JSON.parse(pluginsJson)
      arr.push(JSON.parse(goProcessEntry))
      writeFileSync(pluginsJsonPath, JSON.stringify(arr, null, 2), 'utf-8')
      console.log(`  added GoProcessPlugin to capacitor.plugins.json`)
    } catch (e) {
      console.warn(`  failed to patch capacitor.plugins.json: ${e.message}`)
    }
  }
} else {
  mkdirSync(assetsDir, { recursive: true })
  writeFileSync(pluginsJsonPath, `[${goProcessEntry}]`, 'utf-8')
  console.log(`  created capacitor.plugins.json with GoProcessPlugin`)
}

// --- Debug-only AndroidManifest.xml for Logcat dark theme ---
const debugManifestDir = join(ANDROID_DIR, 'app', 'src', 'debug')
const debugManifestPath = join(debugManifestDir, 'AndroidManifest.xml')
if (!existsSync(debugManifestPath)) {
  mkdirSync(debugManifestDir, { recursive: true })
  const debugManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools">

    <application>
        <activity
            android:name="com.hjq.logcat.LogcatActivity"
            android:configChanges="orientation|screenSize|keyboardHidden"
            android:launchMode="singleInstance"
            android:screenOrientation="portrait"
            android:theme="@style/Theme.AppCompat.NoActionBar"
            tools:node="replace" />

        <meta-data
            android:name="LogcatWindowEntrance"
            android:value="false" />
        <meta-data
            android:name="LogcatNotifyEntrance"
            android:value="true" />
    </application>

</manifest>`
  writeFileSync(debugManifestPath, debugManifest, 'utf-8')
  console.log(`  created ${debugManifestPath}`)
}

if (version) {
  console.log(`  release mode v${version}`)
} else {
  console.log(`  debug mode`)
}
console.log('done')
