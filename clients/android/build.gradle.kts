// Android library layering on top of the JVM SDK at ../java/.
//
// Build:  ./gradlew :quidnug-android:assemble
// Test:   ./gradlew :quidnug-android:test
//
// This module:
//  - Re-exports the full JVM SDK API (Quid, QuidnugClient, ...).
//  - Adds AndroidKeystoreSigner — a Signer implementation backed by
//    the Android Keystore, with StrongBox attestation when available.
//  - Minimum SDK 24 (Android 7.0) — required for StrongBox on newer
//    devices and for modern TLS. compileSdk 34 (Android 14).

plugins {
    id("com.android.library") version "8.3.2"
    kotlin("android") version "1.9.23"
    `maven-publish`
}

android {
    namespace = "com.quidnug.android"
    compileSdk = 34

    defaultConfig {
        minSdk = 24
        consumerProguardFiles("consumer-rules.pro")
    }

    buildFeatures {
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    publishing {
        singleVariant("release") {
            withSourcesJar()
            withJavadocJar()
        }
    }
}

dependencies {
    // Pull in the cross-JVM Quidnug client. Published to Maven Central
    // as com.quidnug:quidnug-client.
    api("com.quidnug:quidnug-client:2.0.0")

    // Kotlin stdlib
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.8.1")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")

    testImplementation("junit:junit:4.13.2")
    androidTestImplementation("androidx.test.ext:junit:1.1.5")
    androidTestImplementation("androidx.test.espresso:espresso-core:3.5.1")
}

publishing {
    publications {
        register<MavenPublication>("release") {
            groupId = "com.quidnug"
            artifactId = "quidnug-android"
            version = "2.0.0"

            afterEvaluate {
                from(components["release"])
            }
        }
    }
}
