import { rmSync, mkdirSync, copyFileSync, cpSync, readdirSync, statSync, existsSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ANDROID_DIR = join(__dirname, '..', 'android')
const OVERLAY_DIR = join(__dirname, '..', 'android-overlay')

console.log('encv-sync-native: syncing native libs to Android project...')

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

console.log('encv-sync-native: done')
