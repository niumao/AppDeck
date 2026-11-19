package com.appdeck2.client

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.Intent
import android.hardware.display.DisplayManager
import android.os.Build
import android.os.IBinder
import android.util.Log
import android.view.Display

class DisplayAutoEnableService : Service() {

    companion object {
        private const val TAG = "DisplayAutoEnableService"
        private const val CHANNEL_ID = "appdeck2_display"
        private const val NOTIFICATION_ID = 2
    }

    @Volatile
    private var running = false
    private var logcatThread: Thread? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, buildNotification("监控外接显示器"))
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (!running) {
            running = true
            // Enable any already-connected but OFF external display
            Thread {
                enableOffDisplays()
            }.start()
            startLogcatMonitor()
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        running = false
        logcatThread?.interrupt()
        logcatThread = null
        super.onDestroy()
    }

    private fun startLogcatMonitor() {
        logcatThread = Thread {
            Log.d(TAG, "Logcat monitor started")
            while (running) {
                var process: Process? = null
                try {
                    // Clear logcat buffer first to avoid processing stale events
                    Runtime.getRuntime().exec(arrayOf("su", "-c", "logcat -c")).waitFor()

                    process = Runtime.getRuntime().exec(
                        arrayOf("su", "-c", "logcat -v brief -s SystemUi--Statusbar:*")
                    )
                    val reader = process.inputStream.bufferedReader()

                    var line: String?
                    while (running) {
                        line = reader.readLine()
                        if (line == null) break
                        if (line.contains("showDialog")) {
                            Log.d(TAG, "Dialog event detected: $line")
                            handleDialogEvent()
                        }
                    }
                } catch (e: InterruptedException) {
                    Log.d(TAG, "Logcat monitor interrupted")
                    break
                } catch (e: Exception) {
                    Log.e(TAG, "Logcat monitor error", e)
                } finally {
                    process?.destroy()
                }

                if (running) {
                    Log.d(TAG, "Logcat process died, restarting in 3s")
                    try {
                        Thread.sleep(3000)
                    } catch (e: InterruptedException) {
                        break
                    }
                }
            }
            Log.d(TAG, "Logcat monitor stopped")
        }.apply {
            name = "DisplayAutoEnable-Logcat"
            isDaemon = true
            start()
        }
    }

    /**
     * Find external display IDs that are OFF via dumpsys display.
     * DisplayManager.getDisplays() does NOT return disabled/OFF displays,
     * so we must parse dumpsys output.
     */
    private fun findOffExternalDisplayIds(): List<Int> {
        try {
            val proc = Runtime.getRuntime().exec(
                arrayOf("su", "-c", "dumpsys display")
            )
            val output = proc.inputStream.bufferedReader().readText()
            proc.waitFor()

            // Match lines like: Display 3 [...state=OFF...]
            val ids = mutableListOf<Int>()
            val regex = Regex("""Display (\d+) \[.*state=OFF""")
            for (match in regex.findAll(output)) {
                val id = match.groupValues[1].toIntOrNull() ?: continue
                if (id != Display.DEFAULT_DISPLAY) {
                    ids.add(id)
                }
            }
            return ids
        } catch (e: Exception) {
            Log.e(TAG, "Error parsing dumpsys display", e)
            return emptyList()
        }
    }

    private fun enableOffDisplays() {
        val offIds = findOffExternalDisplayIds()
        if (offIds.isEmpty()) {
            Log.d(TAG, "No OFF external displays found")
            return
        }
        for (id in offIds) {
            Log.d(TAG, "Found OFF external display $id, enabling")
            enableDisplay(id)
        }
    }

    private fun enableDisplay(displayId: Int) {
        try {
            Runtime.getRuntime().exec(
                arrayOf("su", "-c", "cmd display enable-display $displayId")
            ).waitFor()

            // Dismiss the OxygenOS dialog if still showing
            Runtime.getRuntime().exec(
                arrayOf("su", "-c", "input keyevent KEYCODE_BACK")
            ).waitFor()

            Thread.sleep(1000)
            val dm = getSystemService(Context.DISPLAY_SERVICE) as DisplayManager
            val active = dm.displays.any {
                it.displayId == displayId && it.state == Display.STATE_ON
            }
            if (active) {
                Log.d(TAG, "External display $displayId enabled successfully")
            } else {
                Log.w(TAG, "Display $displayId enabled but state not ON yet")
            }
        } catch (e: InterruptedException) {
            Thread.currentThread().interrupt()
        } catch (e: Exception) {
            Log.e(TAG, "Error enabling display $displayId", e)
        }
    }

    private fun handleDialogEvent() {
        // Brief delay for the display to register
        try { Thread.sleep(500) } catch (_: InterruptedException) { return }
        enableOffDisplays()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "显示器监控",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Monitors for external display dialog"
            }
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(channel)
        }
    }

    private fun buildNotification(text: String): Notification {
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("AppDeck2")
            .setContentText(text)
            .setSmallIcon(R.drawable.ic_notification)
            .setOngoing(true)
            .build()
    }
}
