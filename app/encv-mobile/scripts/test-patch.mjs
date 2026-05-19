// Test: simulate Capacitor 8 app/build.gradle + post-cap-sync.mjs patching
const sample = [
  'plugins {',
  "    id 'com.android.application'",
  '}',
  '',
  'android {',
  '    namespace = "com.encvgo.app"',
  '    compileSdk = rootProject.ext.compileSdkVersion',
  '    defaultConfig {',
  '        applicationId = "com.encvgo.app"',
  '        minSdkVersion rootProject.ext.minSdkVersion',
  '        targetSdkVersion rootProject.ext.targetSdkVersion',
  '        versionCode 1',
  '        versionName "1.0"',
  '    }',
  '    buildTypes {',
  '        release {',
  '            minifyEnabled false',
  '            proguardFiles getDefaultProguardFile("proguard-android.txt"), "proguard-rules.pro"',
  '        }',
  '    }',
  '}',
  '',
  'dependencies {',
  "    implementation fileTree(dir: 'libs', include: ['*.jar'])",
  '}',
].join('\n')

let c = sample
const version = '1.0.1'
const vcode = 101

// Step 1-4
c = c.replace("'com.android.application'", "'com.android.application'\n    'kotlin-android'")
c = c.replace('dependencies {', "dependencies {\n    implementation \"org.jetbrains.kotlin:kotlin-stdlib:2.1.0\"")
c = c.replace(/defaultConfig\s*\{/, "defaultConfig {\n        ndk {\n            abiFilters 'arm64-v8a'\n        }")
c = c.replace(/versionCode\s+\d+/, `versionCode ${vcode}`)
c = c.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`)

// Step 5a: signingConfigs after "android {" — EXACT string anchor
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

// Step 5b: replace release content using EXACT anchor "minifyEnabled false"
c = c.replace(
  "minifyEnabled false",
  "minifyEnabled true\n        shrinkResources true\n        signingConfig signingConfigs.release\n        ENCV_RELEASE_PATCHED = true",
)

console.log('=== Result ===')
console.log(c)
console.log('\n--- Checks ---')
const checks = [
  ['signingConfigs defined', c.includes('signingConfigs')],
  ["storeFile file('../keystore/release.jks')", c.includes("storeFile file('../keystore/release.jks')")],
  ['minifyEnabled true', c.includes('minifyEnabled true')],
  ['shrinkResources true', c.includes('shrinkResources true')],
  ['signingConfig signingConfigs.release', c.includes('signingConfig signingConfigs.release')],
  ['ENCV_RELEASE_PATCHED marker', c.includes('ENCV_RELEASE_PATCHED')],
  ['signingConfigs only once', (c.match(/signingConfigs/g) || []).length === 1],
]
let allPass = true
checks.forEach(([k, v]) => {
  console.log(`  ${v ? '✅' : '❌'} ${k}`)
  if (!v) allPass = false
})
console.log(allPass ? '\nALL PASS' : '\nSOME FAILED')
process.exit(allPass ? 0 : 1)
