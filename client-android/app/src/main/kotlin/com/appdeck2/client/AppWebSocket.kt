package com.appdeck2.client

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.Binder
import android.os.Build
import android.os.IBinder
import android.util.Log
import android.os.BatteryManager
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.hardware.display.DisplayManager
import android.util.Base64
import okhttp3.*
import org.json.JSONObject
import java.util.concurrent.TimeUnit

interface WebSocketCallback {
    fun onWsEvent(type: String, data: String)
}

class AppWebSocket : Service() {

    companion object {
        const val TAG = "AppWebSocket"
        const val CHANNEL_ID = "appdeck2_ws"
        const val NOTIFICATION_ID = 1
        private const val BASE_URL = "xiongmaovr.cn"
    }

    inner class WebSocketBinder : Binder() {
        fun getService(): AppWebSocket = this@AppWebSocket
    }

    private val binder = WebSocketBinder()
    var callback: WebSocketCallback? = null

    private val client = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(0, TimeUnit.MINUTES)
        .build()

    @Volatile private var webSocket: WebSocket? = null
    @Volatile private var cachedDeviceState: String? = null
    @Volatile private var hasEverConnected = false
    @Volatile private var isFreePlay = false
    private var isConnected = false
    private var retryCount = 0
    private val maxBackoff = 30000L
    private val handler = android.os.Handler(android.os.Looper.getMainLooper())
    private var reconnectRunnable: Runnable? = null
    private var missedPongs = 0
    private var gamepadMonitor: GamepadQuitMonitor? = null
    private val GAMEPAD_WATCHDOG_INTERVAL_MS = 30_000L
    private val gamepadWatchdogRunnable = object : Runnable {
        override fun run() {
            gamepadMonitor?.ensureRunning()
            handler.postDelayed(this, GAMEPAD_WATCHDOG_INTERVAL_MS)
        }
    }
    private var disconnectTimerStart: Long = 0L
    private var disconnectCheckRunnable: Runnable? = null
    private val DISCONNECT_TIMEOUT_MS = 10 * 60 * 1000L

    // Low-balance recharge QR prompt
    @Volatile private var lowBalanceNotified = false
    private var lowBalanceCheckRunnable: Runnable? = null
    private val LOW_BALANCE_THRESHOLD_SEC = 180

    override fun onCreate() {
        super.onCreate()
        DeviceIdentity.init(this)
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, buildNotification("AppDeck2 连接中..."))

        gamepadMonitor = GamepadQuitMonitor {
            handler.post { notifyCallback("gamepad_quit", "{}") }
        }
        gamepadMonitor?.start()
        handler.postDelayed(gamepadWatchdogRunnable, GAMEPAD_WATCHDOG_INTERVAL_MS)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (!isConnected) {
            connect()
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder = binder

    fun send(message: String) {
        webSocket?.send(message)
    }

    fun sendAppLaunch(appId: String, appName: String) {
        val msg = JSONObject().apply {
            put("type", "app_launch")
            put("data", JSONObject().apply {
                put("app_id", appId)
                put("app_name", appName)
            })
        }
        webSocket?.send(msg.toString())
    }

    fun sendAppQuit(quitReason: String = "user_quit") {
        val msg = JSONObject().apply {
            put("type", "app_quit")
            put("data", JSONObject().apply {
                put("quit_reason", quitReason)
            })
        }
        webSocket?.send(msg.toString())
    }

    fun getDeviceState(): String? {
        val raw = cachedDeviceState ?: return null
        return try {
            val json = JSONObject(raw)
            json.put("event", "state_sync")
            json.toString()
        } catch (e: Exception) {
            Log.e(TAG, "Failed to adjust cached device state", e)
            raw
        }
    }

    override fun onDestroy() {
        handler.removeCallbacks(gamepadWatchdogRunnable)
        gamepadMonitor?.stop()
        gamepadMonitor = null
        reconnectRunnable?.let { handler.removeCallbacks(it) }
        cancelDisconnectTimer()
        lowBalanceCheckRunnable?.let { handler.removeCallbacks(it) }
        lowBalanceCheckRunnable = null
        webSocket?.close(1000, "Service destroyed")
        client.dispatcher.executorService.shutdown()
        super.onDestroy()
    }

    private fun connect() {
        // Cancel any pending reconnect to prevent stale runnables from firing
        reconnectRunnable?.let { handler.removeCallbacks(it) }
        // Close any existing connection before creating a new one.
        // Without this, each connect() call leaks a server-side goroutine.
        webSocket?.close(1000, "Reconnecting")
        webSocket = null

        val hostId = DeviceIdentity.hostId
        val machineCode = DeviceIdentity.machineCode
        val url = "ws://$BASE_URL:80/api/v1/ws/device?host_id=$hostId&machine_code=$machineCode"

        Log.d(TAG, "Connecting to $url")

        val request = Request.Builder().url(url).build()
        webSocket = client.newWebSocket(request, object : WebSocketListener() {

            override fun onOpen(ws: WebSocket, response: Response) {
                Log.d(TAG, "WebSocket connected")
                isConnected = true
                hasEverConnected = true
                retryCount = 0
                missedPongs = 0
                cancelDisconnectTimer()
                updateNotification("AppDeck2 已连接")

                val registerMsg = JSONObject().apply {
                    put("type", "register")
                    put("data", JSONObject().apply {
                        put("host_id", hostId)
                        put("machine_code", machineCode)
                        put("device_name", "AppDeck2 Android")
                        put("version", "1.0.0")
                    })
                }
                ws.send(registerMsg.toString())
            }

            override fun onMessage(ws: WebSocket, text: String) {
                Log.d(TAG, "Message received: $text")
                try {
                    val json = JSONObject(text)
                    val type = json.optString("type", "")
                    handleMessage(type, json)
                } catch (e: Exception) {
                    Log.e(TAG, "Failed to parse message", e)
                }
            }

            override fun onClosing(ws: WebSocket, code: Int, reason: String) {
                Log.d(TAG, "WebSocket closing: $code $reason")
                ws.close(1000, null)
            }

            override fun onClosed(ws: WebSocket, code: Int, reason: String) {
                Log.d(TAG, "WebSocket closed: $code $reason")
                // Ignore callbacks from replaced connections (prevents reconnect storm)
                if (ws !== webSocket) {
                    Log.d(TAG, "Ignoring onClosed from stale connection")
                    return
                }
                isConnected = false
                updateNotification("AppDeck2 已断开")
                startDisconnectTimer()
                scheduleReconnect()
            }

            override fun onFailure(ws: WebSocket, t: Throwable, response: Response?) {
                Log.e(TAG, "WebSocket failure", t)
                // Ignore callbacks from replaced connections (prevents reconnect storm)
                if (ws !== webSocket) {
                    Log.d(TAG, "Ignoring onFailure from stale connection")
                    return
                }
                isConnected = false
                updateNotification("AppDeck2 重连中...")
                startDisconnectTimer()
                scheduleReconnect()
            }
        })
    }

    private fun getTemperatureData(): Float {
        return try {
            val thermalDir = java.io.File("/sys/class/thermal/")
            if (!thermalDir.exists()) return -1.0f
            val zones = thermalDir.listFiles { f -> f.name.startsWith("thermal_zone") } ?: return -1.0f
            var maxTemp = Int.MIN_VALUE
            for (zone in zones) {
                val tempFile = java.io.File(zone, "temp")
                if (tempFile.exists()) {
                    val raw = tempFile.readText().trim().toIntOrNull()
                    if (raw != null && raw > maxTemp) maxTemp = raw
                }
            }
            if (maxTemp == Int.MIN_VALUE) -1.0f else maxTemp / 1000.0f
        } catch (e: Exception) {
            Log.e(TAG, "Failed to read temperature", e)
            -1.0f
        }
    }

    private fun getBatteryData(): JSONObject {
        val bm = getSystemService(BATTERY_SERVICE) as BatteryManager
        val level = bm.getIntProperty(BatteryManager.BATTERY_PROPERTY_CAPACITY)
        val status = bm.getIntProperty(BatteryManager.BATTERY_PROPERTY_STATUS)
        val state = when (status) {
            BatteryManager.BATTERY_STATUS_CHARGING -> "charging"
            BatteryManager.BATTERY_STATUS_FULL -> "full"
            else -> "discharging"
        }
        return JSONObject().apply {
            put("battery_level", level)
            put("charging_state", state)
            put("device_temp", getTemperatureData())
        }
    }

    private fun handleMessage(type: String, json: JSONObject) {
        when (type) {
            "ping" -> {
                missedPongs = 0
                webSocket?.send(JSONObject().apply {
                    put("type", "pong")
                    put("data", getBatteryData())
                }.toString())
            }
            "device_state_change" -> {
                val dataObj = json.optJSONObject("data")
                if (dataObj != null && dataObj.has("free_play")) {
                    isFreePlay = dataObj.optBoolean("free_play", false)
                }
                val data = dataObj?.toString() ?: "{}"
                cachedDeviceState = data

                // session_end must kill the game even if MainActivity callback is null
                // (Activity may be destroyed by the system while game is in foreground)
                val event = dataObj?.optString("event", "") ?: ""
                if (event == "session_end") {
                    forceKillCurrentGame()
                }

                // On login, restore ALL of this user's Eden games at once
                // (not just whichever one gets launched first), so every
                // game is already in place by the time they pick one.
                // Reports progress so the UI can show a "restoring saves"
                // tip instead of leaving the user guessing why a game they
                // pick mid-restore doesn't have their progress yet.
                if (event == "login") {
                    val userId = dataObj?.optJSONObject("user")?.optString("id", "")
                    if (!userId.isNullOrEmpty()) {
                        SaveSyncManager.restoreForLogin(applicationContext, userId, DeviceIdentity.hostId, DeviceIdentity.machineCode) { completed, total, done ->
                            val statusJson = JSONObject().apply {
                                put("completed", completed)
                                put("total", total)
                                put("done", done)
                            }
                            notifyCallback("save_sync_status", statusJson.toString())
                        }
                    }
                }

                // Low-balance recharge QR timer
                when (event) {
                    "login", "state_sync" -> startLowBalanceCheck()
                    "balance_correction" -> handler.post { checkLowBalance() }
                    "session_end", "connect" -> cancelLowBalanceCheck()
                    "free_play_change" -> if (isFreePlay) cancelLowBalanceCheck()
                }

                notifyCallback("device_state_change", data)
            }
            "client_quit" -> {
                notifyCallback("client_quit", json.optJSONObject("data")?.toString() ?: "{}")
            }
            "system_command" -> {
                val data = json.optJSONObject("data")
                val action = data?.optString("action", "") ?: ""
                when (action) {
                    "reboot" -> {
                        try { Runtime.getRuntime().exec(arrayOf("su", "-c", "reboot")) }
                        catch (e: Exception) { Log.e(TAG, "Reboot failed", e) }
                    }
                    "poweroff" -> {
                        try { Runtime.getRuntime().exec(arrayOf("su", "-c", "reboot", "-p")) }
                        catch (e: Exception) { Log.e(TAG, "Poweroff failed", e) }
                    }
                }
            }
            "save_upload_requested" -> {
                val data = json.optJSONObject("data") ?: return
                val userId = data.optString("user_id", "")
                if (userId.isEmpty()) return
                Thread {
                    SaveSyncManager.stageAndUploadUser(
                        applicationContext, userId,
                        DeviceIdentity.hostId, DeviceIdentity.machineCode
                    )
                }.start()
            }
            "screensaver_config" -> {
                notifyCallback("screensaver_config", json.optJSONObject("data")?.toString() ?: "{}")
            }
            "bypass_charge" -> {
                val data = json.optJSONObject("data")
                val enabled = data?.optBoolean("enabled", false) ?: false
                val value = if (enabled) "1" else "0"
                try {
                    Runtime.getRuntime().exec(arrayOf("su", "-c",
                        "settings put system bypass_charge_state $value"))
                } catch (e: Exception) {
                    Log.e(TAG, "Bypass charge toggle failed", e)
                }
            }
            "power_schedule" -> {
                val data = json.optJSONObject("data") ?: return
                val shutdownEnabled = data.optBoolean("shutdown_enabled", false)
                val shutdownTime = data.optString("shutdown_time", "23:00").replace(":", "")
                val poweronEnabled = data.optBoolean("poweron_enabled", false)
                val poweronTime = data.optString("poweron_time", "08:00").replace(":", "")
                val poweronSwitch = if (poweronEnabled) "1" else "0"
                val shutdownSwitch = if (shutdownEnabled) "1" else "0"
                // 18-char format: poweron block (HHMM1S127) + shutdown block (HHMM0S127)
                val config = "${poweronTime}1${poweronSwitch}127${shutdownTime}0${shutdownSwitch}127"
                Log.d(TAG, "Writing timepower_config: $config")
                Thread {
                    try {
                        Runtime.getRuntime().exec(arrayOf("su", "-c",
                            "settings put secure timepower_config $config")).waitFor()
                        // Open TimePowerSettingsActivity to register the system alarm
                        Runtime.getRuntime().exec(arrayOf("su", "-c",
                            "am start -n com.android.settings/com.oplus.settings.OplusSettingsActivity\$TimePowerSettingsActivity")).waitFor()
                    } catch (e: Exception) {
                        Log.e(TAG, "Power schedule config failed", e)
                    }
                }.start()
            }
            "shell_exec" -> {
                val data = json.optJSONObject("data") ?: return
                val commandId = data.optString("command_id", "")
                val command = data.optString("command", "")
                if (commandId.isEmpty() || command.isEmpty()) return

                Thread {
                    try {
                        val proc = Runtime.getRuntime().exec(arrayOf("su", "-c", command))
                        val stdout = proc.inputStream.bufferedReader().readText()
                        val stderr = proc.errorStream.bufferedReader().readText()
                        proc.waitFor()
                        val exitCode = proc.exitValue()

                        // Cap output at 256KB to prevent WS frame overflow
                        val maxOutput = 256 * 1024
                        val result = JSONObject().apply {
                            put("type", "shell_result")
                            put("data", JSONObject().apply {
                                put("command_id", commandId)
                                put("stdout", stdout.take(maxOutput))
                                put("stderr", stderr.take(maxOutput))
                                put("exit_code", exitCode)
                            })
                        }
                        webSocket?.send(result.toString())
                    } catch (e: Exception) {
                        val result = JSONObject().apply {
                            put("type", "shell_result")
                            put("data", JSONObject().apply {
                                put("command_id", commandId)
                                put("stdout", "")
                                put("stderr", "Error: ${e.message}")
                                put("exit_code", -1)
                            })
                        }
                        webSocket?.send(result.toString())
                    }
                }.start()
            }
            "screen_capture" -> {
                val data = json.optJSONObject("data") ?: return
                val commandId = data.optString("command_id", "")
                val display = data.optString("display", "phone")
                if (commandId.isEmpty()) return

                Thread {
                    try {
                        val tmpFile: String
                        val captureCmd: String

                        if (display == "external") {
                            val dm = getSystemService(DISPLAY_SERVICE) as DisplayManager
                            val extDisplays = dm.getDisplays(DisplayManager.DISPLAY_CATEGORY_PRESENTATION)
                            if (extDisplays.isEmpty()) {
                                val errResult = JSONObject().apply {
                                    put("type", "screen_capture_result")
                                    put("data", JSONObject().apply {
                                        put("command_id", commandId)
                                        put("display", display)
                                        put("error", "No external display connected")
                                    })
                                }
                                webSocket?.send(errResult.toString())
                                return@Thread
                            }
                            tmpFile = "/data/local/tmp/sc_ext.png"
                            // `screencap -d` takes the *physical* display ID reported by
                            // `dumpsys SurfaceFlinger --display-id`, not DisplayManager's
                            // logical display ID used elsewhere (e.g. launchDisplayId). Passing
                            // the logical ID silently matches no display, leaving tmpFile
                            // empty/stale and making decodeFile() below return null. The
                            // built-in panel is always "HWC display 0"; the external one is
                            // whatever physical ID remains.
                            captureCmd = "PHYS_ID=\$(dumpsys SurfaceFlinger --display-id | grep -v 'HWC display 0' | grep -oE '[0-9]+' | head -1); " +
                                "if [ -n \"\$PHYS_ID\" ]; then screencap -d \$PHYS_ID -p $tmpFile; else screencap -p $tmpFile; fi"
                        } else {
                            tmpFile = "/data/local/tmp/sc_phone.png"
                            captureCmd = "screencap -p $tmpFile"
                        }

                        Runtime.getRuntime().exec(arrayOf("su", "-c", captureCmd)).waitFor()

                        var bitmap = BitmapFactory.decodeFile(tmpFile)
                            ?: throw Exception("Failed to decode screenshot")

                        // Resize to max 960px width
                        if (bitmap.width > 960) {
                            val scale = 960.0 / bitmap.width
                            val newHeight = (bitmap.height * scale).toInt()
                            val scaled = Bitmap.createScaledBitmap(bitmap, 960, newHeight, true)
                            bitmap.recycle()
                            bitmap = scaled
                        }

                        // Compress JPEG 70%
                        val outputStream = java.io.ByteArrayOutputStream()
                        bitmap.compress(Bitmap.CompressFormat.JPEG, 70, outputStream)
                        val jpegBytes = outputStream.toByteArray()

                        val w = bitmap.width
                        val h = bitmap.height
                        bitmap.recycle()

                        // Base64 encode
                        val base64Str = Base64.encodeToString(jpegBytes, Base64.NO_WRAP)

                        // Clean up temp file
                        Runtime.getRuntime().exec(arrayOf("su", "-c", "rm -f $tmpFile")).waitFor()

                        val result = JSONObject().apply {
                            put("type", "screen_capture_result")
                            put("data", JSONObject().apply {
                                put("command_id", commandId)
                                put("image_base64", base64Str)
                                put("display", display)
                                put("width", w)
                                put("height", h)
                            })
                        }
                        webSocket?.send(result.toString())
                    } catch (e: Exception) {
                        Log.e(TAG, "Screen capture failed", e)
                        val result = JSONObject().apply {
                            put("type", "screen_capture_result")
                            put("data", JSONObject().apply {
                                put("command_id", commandId)
                                put("display", display)
                                put("error", e.message ?: "Screen capture failed")
                            })
                        }
                        webSocket?.send(result.toString())
                    }
                }.start()
            }
        }
    }

    private fun notifyCallback(type: String, data: String) {
        handler.post { callback?.onWsEvent(type, data) }
    }

    private fun scheduleReconnect() {
        reconnectRunnable?.let { handler.removeCallbacks(it) }
        val delay = minOf((1000L * (1L shl minOf(retryCount, 5))), maxBackoff)
        retryCount++
        Log.d(TAG, "Reconnecting in ${delay}ms (attempt $retryCount)")
        reconnectRunnable = Runnable { connect() }
        handler.postDelayed(reconnectRunnable!!, delay)
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "AppDeck2 连接",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "WebSocket connection status"
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

    private fun updateNotification(text: String) {
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, buildNotification(text))
    }

    private fun startDisconnectTimer() {
        // Only start once — don't reset on each reconnect attempt
        if (disconnectCheckRunnable != null) return
        disconnectTimerStart = android.os.SystemClock.elapsedRealtime()
        disconnectCheckRunnable = object : Runnable {
            override fun run() {
                val elapsed = android.os.SystemClock.elapsedRealtime() - disconnectTimerStart
                if (elapsed >= DISCONNECT_TIMEOUT_MS) {
                    onDisconnectTimeout()
                } else {
                    handler.postDelayed(this, 30_000L)
                }
            }
        }
        handler.postDelayed(disconnectCheckRunnable!!, 30_000L)
    }

    private fun cancelDisconnectTimer() {
        disconnectCheckRunnable?.let { handler.removeCallbacks(it) }
        disconnectCheckRunnable = null
    }

    /**
     * Kill the currently running game directly from the service.
     * Reads the game package from the file written by GameLauncher,
     * so this works even when MainActivity is destroyed.
     */
    private fun forceKillCurrentGame() {
        Thread {
            try {
                val pkgFile = java.io.File(GamepadQuitMonitor.CURRENT_GAME_PATH)
                if (!pkgFile.exists()) return@Thread
                val pkg = pkgFile.readText().trim()
                if (pkg.isEmpty()) return@Thread
                Log.d(TAG, "session_end: force-stopping game $pkg from service")
                val proc = Runtime.getRuntime().exec(arrayOf("su", "-c", "am", "force-stop", pkg))
                proc.waitFor()
                pkgFile.delete()
                // Bring MainActivity to front
                val intent = android.content.Intent(this, MainActivity::class.java).apply {
                    addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK or
                            android.content.Intent.FLAG_ACTIVITY_REORDER_TO_FRONT)
                }
                startActivity(intent)
            } catch (e: Exception) {
                Log.e(TAG, "forceKillCurrentGame failed", e)
            }
        }.start()
    }

    private fun startLowBalanceCheck() {
        cancelLowBalanceCheck()
        lowBalanceNotified = false
        lowBalanceCheckRunnable = object : Runnable {
            override fun run() {
                checkLowBalance()
                handler.postDelayed(this, 10_000L)
            }
        }
        handler.post(lowBalanceCheckRunnable!!)
    }

    private fun cancelLowBalanceCheck() {
        lowBalanceCheckRunnable?.let { handler.removeCallbacks(it) }
        lowBalanceCheckRunnable = null
        if (lowBalanceNotified) {
            lowBalanceNotified = false
            notifyCallback("low_balance_clear", "{}")
        }
    }

    private fun checkLowBalance() {
        val raw = cachedDeviceState ?: return
        if (isFreePlay) return
        try {
            val json = JSONObject(raw)
            if (json.optString("state") != "logged_in") return

            val serverTs = json.optLong("server_ts", 0L)
            if (serverTs == 0L) return
            val elapsedSec = (System.currentTimeMillis() / 1000.0) - serverTs
            val tenantMinutes = json.optInt("tenant_balance_minutes", 0)
            val globalCents = json.optInt("global_balance_cents", 0)
            val planRateCents = json.optInt("plan_rate_cents", 0)
            val planUnitSeconds = json.optInt("plan_unit_seconds", 3600)
            val otherSessions = json.optInt("other_session_count", 0)
            val totalSessions = 1 + otherSessions

            val tenantSeconds = (tenantMinutes * 60.0) / totalSessions
            var globalSeconds = 0.0
            if (planRateCents > 0 && planUnitSeconds > 0) {
                val ratePerSecond = planRateCents.toDouble() / planUnitSeconds
                globalSeconds = (globalCents / ratePerSecond) / totalSessions
            }
            // baseSeconds is the remaining balance as of serverTs; the overlay
            // countdown re-derives remaining time from (baseSeconds, serverTs)
            // on every tick instead of locally decrementing, so it can't drift
            // from the server-truth countdown shown in the H5 right panel.
            val baseSeconds = tenantSeconds + globalSeconds
            val adjusted = maxOf(0, (baseSeconds - elapsedSec).toInt())

            if (adjusted <= LOW_BALANCE_THRESHOLD_SEC && !lowBalanceNotified) {
                lowBalanceNotified = true
                notifyCallback("low_balance", """{"remaining_seconds":$adjusted,"server_ts":$serverTs,"base_seconds":$baseSeconds}""")
            } else if (adjusted > LOW_BALANCE_THRESHOLD_SEC && lowBalanceNotified) {
                lowBalanceNotified = false
                notifyCallback("low_balance_clear", "{}")
            }
        } catch (e: Exception) {
            Log.e(TAG, "checkLowBalance failed", e)
        }
    }

    private fun onDisconnectTimeout() {
        if (!hasEverConnected) return  // Still on pending screen
        if (isFreePlay) return  // No enforcement needed for free-play

        Log.w(TAG, "10-min disconnect timeout, forcing offline state")
        cachedDeviceState = JSONObject().apply {
            put("state", "offline")
            put("event", "connect")
        }.toString()
        forceKillCurrentGame()
        notifyCallback("disconnect_timeout", "{}")
    }
}
