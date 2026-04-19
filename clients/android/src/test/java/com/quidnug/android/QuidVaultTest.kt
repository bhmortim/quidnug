package com.quidnug.android

import org.junit.Assert.*
import org.junit.Test

/**
 * Placeholder — real Android tests run on-device / emulator and
 * exercise AndroidKeystoreSigner (which needs BOLOS-backed
 * device APIs). This JVM-level test validates that the module
 * compiles and that QuidnugAndroidClient.create returns a usable
 * instance with the right types.
 */
class CompilationSmokeTest {

    @Test
    fun createBuildsClient() {
        val client = QuidnugAndroidClient.create("http://localhost:8080")
        assertNotNull(client)
    }
}
