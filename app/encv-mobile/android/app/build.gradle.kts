import com.combo.aar2apk.PackageBuildType
import org.jetbrains.kotlin.gradle.tasks.KotlinCompile

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    alias(libs.plugins.combolite.aar2apk)
}

android {
    namespace = "com.encvgo.app"
    compileSdk = libs.versions.compileSdk.get().toInt()

    defaultConfig {
        applicationId = "com.encvgo.app"
        minSdk = libs.versions.minSdk.get().toInt()
        targetSdk = libs.versions.targetSdk.get().toInt()
        versionCode = 1
        versionName = "1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        aaptOptions {
            ignoreAssetsPattern = "!.svn:!.git:!.ds_store:!*.scc:.*:!CVS:!thumbs.db:!picasa.ini:!*~"
        }

        buildConfigField("boolean", "USE_LYNX_PLAYER", "true")

        ndk {
            abiFilters += setOf("arm64-v8a")
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android.txt"),
                "proguard-rules.pro"
            )
        }
    }

    compileOptions {
        targetCompatibility = JavaVersion.VERSION_21
        sourceCompatibility = JavaVersion.VERSION_21
    }

    buildFeatures {
        buildConfig = true
        compose = true
    }

    sourceSets {
        getByName("main") {
            jniLibs.srcDirs("src/main/jniLibs")
        }
    }

    packaging {
        jniLibs {
            useLegacyPackaging = true
            pickFirsts += setOf("**/*.so")
        }
        resources {
            pickFirsts += setOf("**/*.so")
        }
    }
}

packagePlugins {
    enabled.set(true)
    val isReleaseBuild = gradle.startParameter.taskNames.any { it.lowercase().contains("release") }
    buildType.set(if (isReleaseBuild) PackageBuildType.RELEASE else PackageBuildType.DEBUG)
    pluginsDir.set("plugins")
}

repositories {
    flatDir {
        dirs("../capacitor-cordova-android-plugins/src/main/libs", "libs")
    }
}

dependencies {
    implementation(fileTree(mapOf("include" to listOf("*.jar"), "dir" to "libs")))
    implementation(libs.androidx.appcompat)
    implementation(libs.androidx.coordinatorlayout)
    implementation(libs.androidx.core.splashscreen)
    implementation(project(":capacitor-android"))
    testImplementation(libs.junit)
    testImplementation(libs.mockito.core)
    testImplementation(libs.mockito.kotlin)
    testImplementation(libs.kotlin.test)
    androidTestImplementation(libs.androidx.junit)
    androidTestImplementation(libs.androidx.espresso.core)
    implementation(project(":capacitor-cordova-android-plugins"))
    implementation(libs.kotlin.stdlib)
    debugImplementation(libs.logcat)

    implementation(libs.lynx)
    implementation(libs.lynx.jssdk)
    implementation(libs.lynx.trace)
    implementation(libs.primjs)
    implementation(libs.lynx.service.image)
    implementation(libs.lynx.service.log)
    implementation(libs.lynx.service.http)
    implementation(libs.lynx.service.devtool)
    implementation(libs.lynx.devtool)
    implementation(libs.fresco)
    implementation(libs.fresco.animated.gif)
    implementation(libs.fresco.animated.webp)
    implementation(libs.fresco.webpsupport)
    implementation(libs.fresco.animated.base)
    implementation(libs.okhttp)

    implementation(libs.combolite.core)
    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.ui.tooling.preview)
    implementation(libs.compose.material3)
    implementation(libs.androidx.activity.compose)
}

apply(from = "capacitor.build.gradle")

try {
    val servicesJSON = file("google-services.json")
    if (servicesJSON.exists() && servicesJSON.readText().isNotEmpty()) {
        apply(plugin = "com.google.gms.google-services")
    }
} catch (e: Exception) {
    logger.info("google-services.json not found, google-services plugin not applied. Push Notifications won't work")
}

tasks.withType<KotlinCompile>().configureEach {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_21)
    }
}

android.sourceSets {
    getByName("main") {
        java.srcDirs("src/main/java")
    }
}
