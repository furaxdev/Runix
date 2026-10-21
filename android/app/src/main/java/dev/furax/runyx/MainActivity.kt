package dev.furax.runyx

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

/**
 * Placeholder entry point. Device pairing (QR + mDNS), remote model use,
 * and the rest of the Android feature set land in M5 — see
 * docs/ROADMAP.md item 26-28. This exists so the module structure and
 * Gradle wiring are in place before that work starts.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            RunyxPlaceholder()
        }
    }
}

@Composable
fun RunyxPlaceholder() {
    MaterialTheme {
        Surface(modifier = Modifier.fillMaxSize()) {
            Column(modifier = Modifier.padding(24.dp)) {
                Text("Runyx — pairing and chat UI land in milestone M5.")
            }
        }
    }
}
