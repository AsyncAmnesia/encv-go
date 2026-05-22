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

// --- Gradle patching ---
console.log('encv-sync-native: patching Gradle files...')

function patchFile(filePath, transformer) {
  if (!existsSync(filePath)) {
    console.warn(`  gradle: file not found: ${filePath}`)
    return
  }
  const content = readFileSync(filePath, 'utf-8')
  const modified = transformer(content)
  if (modified !== content) {
    writeFileSync(filePath, modified, 'utf-8')
    console.log(`  gradle: patched ${filePath}`)
  }
}

// Root build.gradle
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

// App build.gradle
patchFile(join(ANDROID_DIR, 'app', 'build.gradle'), (c) => {
  if (!c.includes('kotlin-android') && !c.includes('org.jetbrains.kotlin.android')) {
    c = c.replace(
      "apply plugin: 'com.android.application'",
      "apply plugin: 'com.android.application'\napply plugin: 'kotlin-android'"
    )
  }

  if (!c.includes('kotlin-stdlib')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    implementation \"org.jetbrains.kotlin:kotlin-stdlib:2.1.0\""
    )
  }
  if (!c.includes('Logcat')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    debugImplementation 'com.github.getActivity:Logcat:13.0'"
    )
  }

  if (!c.includes('USE_LYNX_PLAYER')) {
    c = c.replace(
      /defaultConfig\s*\{/,
      "defaultConfig {\n        buildConfigField \"boolean\", \"USE_LYNX_PLAYER\", \"true\""
    )
  }

  if (!c.includes('buildConfig = true') && !c.includes('buildConfig true')) {
    if (c.includes('buildFeatures')) {
      c = c.replace(/buildFeatures\s*\{/, "buildFeatures {\n        buildConfig = true")
    } else {
      c = c.replace(/android\s*\{/, "android {\n    buildFeatures {\n        buildConfig = true\n    }")
    }
  }

  if (!c.includes('abiFilters') || !c.includes('arm64-v8a')) {
    c = c.replace(
      /defaultConfig\s*\{/,
      "defaultConfig {\n        ndk {\n            abiFilters 'arm64-v8a'\n        }"
    )
  }

  if (!c.includes('compileOptions')) {
    c = c.replace(
      /android\s*\{/,
      "android {\n    compileOptions {\n        targetCompatibility JavaVersion.VERSION_21\n        sourceCompatibility JavaVersion.VERSION_21\n    }"
    )
  }

  if (!c.includes('org.lynxsdk.lynx')) {
    c = c.replace(
      'dependencies {',
      "dependencies {\n    implementation 'org.lynxsdk.lynx:lynx:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-jssdk:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-trace:3.7.0'\n    implementation 'org.lynxsdk.lynx:primjs:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-service-image:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-service-log:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-service-http:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-service-devtool:3.7.0'\n    implementation 'org.lynxsdk.lynx:lynx-devtool:3.7.0'\n    implementation 'com.facebook.fresco:fresco:2.3.0'\n    implementation 'com.facebook.fresco:animated-gif:2.3.0'\n    implementation 'com.facebook.fresco:animated-webp:2.3.0'\n    implementation 'com.facebook.fresco:webpsupport:2.3.0'\n    implementation 'com.facebook.fresco:animated-base:2.3.0'\n    implementation 'com.squareup.okhttp3:okhttp:4.9.0'"
    )
  }

  c = c.replace(/\s*implementation\s*\(?['"]io\.github\.abdallahmehiz:mpv[^'"]*['"][\s\S]*?\)?/g, '')

  if (!c.includes('jniLibs.srcDirs')) {
    c = c.replace(
      /android\s*\{/,
      "android {\n    sourceSets {\n        main {\n            jniLibs.srcDirs = ['src/main/jniLibs']\n        }\n    }"
    )
  }

  if (!c.includes('useLegacyPackaging')) {
    c = c.replace(
      /android\s*\{/,
      "android {\n    packaging {\n        jniLibs {\n            useLegacyPackaging = true\n            pickFirsts += ['**/*.so']\n        }\n        resources {\n            pickFirsts += ['**/*.so']\n        }\n    }"
    )
  }

  if (!c.includes('jvmTarget') && c.includes('kotlin-android')) {
    c += `\ntasks.withType(org.jetbrains.kotlin.gradle.tasks.KotlinCompile).configureEach {\n    kotlinOptions {\n        jvmTarget = "21"\n    }\n}\n`
  }

  if (!c.includes("main.java.srcDirs += 'src/main/java'") && c.includes('kotlin-android')) {
    c += `\nandroid.sourceSets {\n    main.java.srcDirs += 'src/main/java'\n}\n`
  }

  const version = process.env.ENCV_VERSION || ''
  if (version) {
    const vcode = parseInt(version.replace(/\./g, '')) || 1
    c = c.replace(/versionCode\s+\d+/, `versionCode ${vcode}`)
    c = c.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`)
  }

  return c
})

console.log('encv-sync-native: done')
