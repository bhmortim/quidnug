package com.quidnug.examples

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.lifecycle.lifecycleScope
import com.quidnug.android.AndroidKeystoreSigner
import com.quidnug.android.QuidnugAndroidClient
import com.quidnug.android.QuidVault
import com.quidnug.client.Quid
import com.quidnug.client.QuidnugClient
import kotlinx.coroutines.launch

/**
 * Example Activity demonstrating the full auth flow:
 *
 *  1. On first launch, create a Keystore-backed quid (StrongBox if
 *     available). The private key NEVER enters the JVM heap.
 *  2. Register the quid with the Quidnug node (name = device model,
 *     home_domain = app identifier).
 *  3. On every login, emit a signed LOGIN event onto the quid's
 *     stream with IP, device info, and timestamp.
 *  4. Display the last 20 events in a list view.
 */
class MobileAuthActivity : ComponentActivity() {

    private val client = QuidnugAndroidClient.create("https://api.myapp.example.com")
    private val vault by lazy { QuidVault(this) }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        lifecycleScope.launch {
            // 1. Ensure we have a Quid
            val quid = vault.load("user") ?: run {
                val fresh = vault.generate("user")
                client.registerIdentity(
                    signer = fresh,
                    name = "device-${android.os.Build.MODEL}",
                    homeDomain = "myapp.users"
                )
                fresh
            }

            // 2. Record a LOGIN event
            client.emitEvent(
                signer = quid,
                subjectId = quid.id(),
                subjectType = "QUID",
                eventType = "LOGIN",
                domain = "myapp.users",
                payload = mapOf(
                    "deviceModel" to android.os.Build.MODEL,
                    "deviceManufacturer" to android.os.Build.MANUFACTURER,
                    "androidVersion" to android.os.Build.VERSION.RELEASE,
                )
            )

            // 3. Fetch audit log
            val events = client.streamEvents(quid.id(), "myapp.users", limit = 20)
            events.forEach { println("${it.sequence} ${it.eventType} @ ${it.timestamp}") }
        }
    }
}
