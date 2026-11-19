package com.appdeck2.client

import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.DocumentsContract
import android.util.Log
import kotlinx.coroutines.*
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject

class GameLauncher(private val activity: MainActivity) {

    companion object {
        private const val TAG = "GameLauncher"
        private const val BASE_URL = "http://xiongmaovr.cn/api/v1"

        // Eden (yuzu-based Switch emulator, repackaged as com.miHoYo.Yuanshen)
        const val EDEN_PACKAGE = "com.miHoYo.Yuanshen"
        private const val EDEN_EMULATION_ACTIVITY = "org.yuzu.yuzu_emu.activities.EmulationActivity"
        private const val EDEN_GAMES_DIR = "Documents/swi"

        // RetroArch
        private const val RETROARCH_PACKAGE = "com.retroarch.aarch64"
        private const val RETROARCH_ACTIVITY = "com.retroarch.browser.retroactivity.RetroActivityFuture"
        private const val RETROARCH_ROM_BASE = "/storage/emulated/0/RetroArch/gam"
        private const val RETROARCH_CORE_BASE = "/data/user/0/com.retroarch.aarch64/cores"
        private val DIR_TO_CORE = mapOf(
            "FC" to "nestopia",
            "SFC" to "snes9x",
            "ARCADE" to "fbalpha2012",
            "sega-md" to "genesis_plus_gx",
            "Sega-32X" to "picodrive",
            "PCE" to "mednafen_pce_fast",
            "NGPC" to "mednafen_ngp",
            "NDS" to "melonds",
        )

        // Moonlight
        private const val MOONLIGHT_PACKAGE = "com.limelight"

        // SharedPreferences key for SAF tree URI
        private const val PREFS_NAME = "game_launcher"
        private const val PREF_SAF_TREE_URI = "saf_tree_uri"
    }

    data class GameLaunchData(
        val id: String,
        val name: String,
        val launchType: String,
        val launchParam: String,
        val retroarchCore: String,
        val retroarchRom: String,
        val authToken: String,
        val sessionId: String,
        val freePlay: Boolean,
        val hostId: String,
        val machineCode: String
    )

    var isGameRunning = false
        private set
    private var currentLaunchData: GameLaunchData? = null
    private var moonlightHostId: String? = null
    var currentGamePackage: String? = null
        private set
    var webSocket: AppWebSocket? = null

    // External display support
    var externalDisplayId: Int? = null
    var launchedOnExternalDisplay = false
        private set
    private var processMonitorThread: Thread? = null
    @Volatile
    private var sessionEndedDuringLaunch = false


    // Pending Eden launch data (waiting for SAF permission)
    private var pendingEdenLaunch: GameLaunchData? = null

    private val httpClient = OkHttpClient.Builder()
        .connectTimeout(5, java.util.concurrent.TimeUnit.SECONDS)
        .readTimeout(10, java.util.concurrent.TimeUnit.SECONDS)
        .build()

    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    private val jsonType = "application/json; charset=utf-8".toMediaType()

    // Single-quotes a value for safe interpolation into a `su -c "<shell string>"` command,
    // since su re-parses that string through a shell rather than passing args directly.
    private fun shellQuote(value: String): String {
        return "'" + value.replace("'", "'\\''") + "'"
    }

    private fun startActivityOnDisplay(intent: Intent, displayId: Int?) {
        if (displayId != null) {
            val options = android.app.ActivityOptions.makeBasic()
            options.launchDisplayId = displayId
            activity.startActivity(intent, options.toBundle())
        } else {
            activity.startActivity(intent)
        }
    }

    private fun writeCurrentGamePackage(pkg: String) {
        try {
            val proc = Runtime.getRuntime().exec(arrayOf("su", "-c",
                "echo '$pkg' > ${GamepadQuitMonitor.CURRENT_GAME_PATH}"))
            proc.waitFor()
        } catch (e: Exception) {
            Log.w(TAG, "Failed to write current game package", e)
        }
    }

    private fun clearCurrentGamePackage() {
        try {
            val proc = Runtime.getRuntime().exec(arrayOf("su", "-c",
                "rm -f ${GamepadQuitMonitor.CURRENT_GAME_PATH}"))
            proc.waitFor()
        } catch (_: Exception) {}
    }

    private fun startProcessMonitor(packageName: String) {
        processMonitorThread = Thread {
            try {
                Thread.sleep(3000) // wait for app to start
                while (isGameRunning) {
                    Thread.sleep(1500)
                    val running = try {
                        val proc = Runtime.getRuntime().exec(arrayOf("su", "-c", "pidof $packageName"))
                        proc.waitFor() == 0
                    } catch (e: Exception) { false }
                    if (!running) {
                        activity.runOnUiThread { onGameReturned() }
                        break
                    }
                }
            } catch (e: InterruptedException) {
                // Thread was interrupted during shutdown, exit silently
            }
        }.apply { isDaemon = true; start() }
    }

    fun launch(jsonData: String) {
        Log.d(TAG, "launch() called, jsonData=$jsonData")
        if (isGameRunning) {
            Log.w(TAG, "Game already running, ignoring launch")
            notifyError("游戏正在运行中")
            return
        }
        val data = parseJson(jsonData)
        if (data == null) {
            Log.e(TAG, "Failed to parse launch JSON")
            notifyError("启动参数错误")
            return
        }
        Log.d(TAG, "Parsed: name=${data.name}, type=${data.launchType}, param=${data.launchParam}")
        currentLaunchData = data
        sessionEndedDuringLaunch = false
        // Notify server of app launch via WebSocket
        webSocket?.sendAppLaunch(data.id, data.name)
        scope.launch {
            try {
                when (data.launchType) {
                    "moonlight" -> launchMoonlight(data)
                    "retroarch" -> launchRetroArch(data)
                    "eden" -> launchEden(data)
                    "android" -> launchAndroidApp(data)
                    else -> launchMoonlight(data)
                }
            } catch (e: Exception) {
                Log.e(TAG, "Launch failed: type=${data.launchType}, param=${data.launchParam}", e)
                isGameRunning = false
                currentLaunchData = null
                currentGamePackage = null
                notifyError(e.message ?: "启动失败")
            }
        }
    }

    private suspend fun launchMoonlight(data: GameLaunchData) {
        // 1. Acquire moonlight host
        val acquireBody = JSONObject().apply {
            put("device_id", "${data.hostId}:${data.machineCode}")
        }.toString().toRequestBody(jsonType)

        val acquireRequest = Request.Builder()
            .url("$BASE_URL/client/moonlight/acquire")
            .post(acquireBody)
            .build()

        val acquireResponse = httpClient.newCall(acquireRequest).execute()
        if (!acquireResponse.isSuccessful) {
            throw Exception("获取云电脑失败: ${acquireResponse.code}")
        }
        val acquireJson = JSONObject(acquireResponse.body!!.string())
        val hostData = acquireJson.optJSONObject("data")
            ?: throw Exception("获取云电脑失败: 无效响应")
        moonlightHostId = hostData.optString("id")
        val host = hostData.optString("host", "")
        val port = hostData.optInt("port", 47989)

        if (host.isEmpty()) {
            throw Exception("获取云电脑失败: 主机地址为空")
        }

        if (sessionEndedDuringLaunch) {
            releaseMoonlightHost(data)
            currentLaunchData = null; currentGamePackage = null; moonlightHostId = null
            return
        }

        // 2. Launch Moonlight with direct stream URI
        isGameRunning = true
        currentGamePackage = MOONLIGHT_PACKAGE
        writeCurrentGamePackage(MOONLIGHT_PACKAGE)
        val displayId = externalDisplayId
        launchedOnExternalDisplay = displayId != null
        withContext(Dispatchers.Main) {
            val appName = data.launchParam.ifEmpty { "Desktop" }
            val uri = Uri.Builder()
                .scheme("moonlight")
                .authority("stream")
                .appendPath(host)
                .appendPath(appName)
                .build()

            val intent = Intent(Intent.ACTION_VIEW, uri)
            try {
                startActivityOnDisplay(intent, displayId)
            } catch (e: Exception) {
                Log.w(TAG, "Failed to launch Moonlight with URI, falling back to package launch", e)
                val fallbackIntent = activity.packageManager.getLaunchIntentForPackage(MOONLIGHT_PACKAGE)
                    ?: throw Exception("Moonlight 未安装")
                startActivityOnDisplay(fallbackIntent, displayId)
            }
            if (displayId != null) {
                startProcessMonitor(MOONLIGHT_PACKAGE)
            }
        }
    }

    private suspend fun launchRetroArch(data: GameLaunchData) {
        if (sessionEndedDuringLaunch) {
            currentLaunchData = null; currentGamePackage = null
            return
        }

        // 1. Launch RetroArch via root am start (avoids Oppo windowed mode)
        isGameRunning = true
        currentGamePackage = RETROARCH_PACKAGE
        writeCurrentGamePackage(RETROARCH_PACKAGE)
        val displayId = externalDisplayId
        launchedOnExternalDisplay = displayId != null

        // Handle both absolute paths and relative paths for ROM
        val rom = if (data.retroarchRom.startsWith("/")) {
            data.retroarchRom
        } else {
            "$RETROARCH_ROM_BASE/${data.retroarchRom}"
        }
        // Resolve core: explicit value, auto-detect from ROM dir, or fail
        val coreName = data.retroarchCore.ifEmpty {
            val dir = data.retroarchRom.substringBefore('/')
            DIR_TO_CORE[dir] ?: throw IllegalStateException(
                "Cannot detect RetroArch core for ROM path: ${data.retroarchRom}"
            )
        }
        val core = if (coreName.startsWith("/")) {
            coreName
        } else {
            "$RETROARCH_CORE_BASE/${coreName}_libretro_android.so"
        }

        val component = "$RETROARCH_PACKAGE/$RETROARCH_ACTIVITY"

        // su -c re-joins everything after it and re-parses it through a shell, so
        // arguments containing spaces (most Chinese ROM filenames) must be quoted here
        // or they get word-split before reaching `am start`.
        val amStartCmd = if (displayId != null) {
            "am start --display $displayId -n $component -e ROM ${shellQuote(rom)} -e LIBRETRO ${shellQuote(core)}"
        } else {
            "am start -n $component -e ROM ${shellQuote(rom)} -e LIBRETRO ${shellQuote(core)}"
        }
        val amCmd = arrayOf("su", "-c", amStartCmd)

        try {
            val proc = Runtime.getRuntime().exec(amCmd)
            val exitCode = proc.waitFor()
            if (exitCode != 0) {
                isGameRunning = false
                throw Exception("RetroArch 未安装")
            }
            Log.d(TAG, "Launched RetroArch via root am start: ROM=$rom CORE=$core")
            // Root am start doesn't trigger normal activity transition,
            // so explicitly move AppDeck to back so game appears in front
            if (displayId == null) {
                withContext(Dispatchers.Main) {
                    activity.moveTaskToBack(true)
                }
            }
            if (displayId != null) {
                startProcessMonitor(RETROARCH_PACKAGE)
            }
        } catch (e: Exception) {
            isGameRunning = false
            throw Exception("RetroArch 启动失败: ${e.message}")
        }
    }

    private suspend fun launchEden(data: GameLaunchData) {
        Log.d(TAG, "launchEden: param=${data.launchParam}")

        // 1. Check if Eden is installed
        try {
            val pkgInfo = activity.packageManager.getPackageInfo(EDEN_PACKAGE, 0)
            Log.d(TAG, "Eden found: pkg=$EDEN_PACKAGE, version=${pkgInfo.versionName}")
        } catch (e: Exception) {
            Log.e(TAG, "Eden not installed: $EDEN_PACKAGE", e)
            throw Exception("Eden 模拟器未安装，请先安装 Eden")
        }

        if (data.launchParam.isEmpty()) {
            Log.e(TAG, "Eden launchParam is empty")
            throw Exception("Eden 启动参数缺失")
        }

        // 2. Check SAF tree permission
        val treeUri = getSavedTreeUri()
        if (treeUri == null) {
            Log.d(TAG, "No SAF tree permission, requesting...")
            pendingEdenLaunch = data
            withContext(Dispatchers.Main) {
                activity.requestSafTreePermission()
            }
            return // Will resume in onSafTreeGranted()
        }

        if (sessionEndedDuringLaunch) {
            currentLaunchData = null; currentGamePackage = null
            return
        }

        // 3. Launch Eden with SAF document URI
        launchEdenWithTreeUri(data, treeUri)
    }

    /** Reads user_id from the WS-cached device state, if any. */
    private fun currentUserId(): String? {
        val raw = webSocket?.getDeviceState() ?: return null
        return try {
            val json = JSONObject(raw)
            // Most device_state_change payloads (login, etc.) nest the id
            // under "user": {"id": ...}; the reconnect-fallback path uses a
            // flat "user_id" instead. Check both.
            json.optJSONObject("user")?.optString("id", "")?.ifEmpty { null }
                ?: json.optString("user_id", "").ifEmpty { null }
        } catch (e: Exception) {
            null
        }
    }

    private suspend fun launchEdenWithTreeUri(data: GameLaunchData, treeUri: Uri) {
        // Safety net: if login-time restore for this user hasn't landed yet,
        // block here and do it now so Eden never starts against a stale or
        // half-restored save. No-op if login-time restore already finished.
        currentUserId()?.let { userId ->
            withContext(Dispatchers.IO) {
                SaveSyncManager.ensureRestoredForLaunch(activity, userId, data.hostId, data.machineCode)
            }
        }

        isGameRunning = true
        currentGamePackage = EDEN_PACKAGE
        writeCurrentGamePackage(EDEN_PACKAGE)
        val displayId = externalDisplayId
        launchedOnExternalDisplay = displayId != null

        // Build SAF document URI
        val absolutePrefix = "/storage/emulated/0/$EDEN_GAMES_DIR/"
        val relativePath = if (data.launchParam.startsWith(absolutePrefix)) {
            data.launchParam.removePrefix(absolutePrefix)
        } else if (data.launchParam.startsWith("/storage/emulated/0/")) {
            data.launchParam.removePrefix("/storage/emulated/0/")
        } else {
            data.launchParam
        }
        val treeDocId = android.provider.DocumentsContract.getTreeDocumentId(treeUri)
        val documentId = "$treeDocId/$relativePath"
        val documentUri = android.provider.DocumentsContract.buildDocumentUriUsingTree(treeUri, documentId)

        Log.d(TAG, "Eden document URI: $documentUri")

        // Launch via root am start (separate task, avoids Oppo same-task focus issues)
        val component = "$EDEN_PACKAGE/$EDEN_EMULATION_ACTIVITY"
        val amCmd = if (displayId != null) {
            arrayOf("su", "-c", "am", "start", "--display", displayId.toString(),
                "-n", component, "-a", "android.intent.action.VIEW",
                "-t", "application/octet-stream",
                "-d", documentUri.toString(),
                "--grant-read-uri-permission")
        } else {
            arrayOf("su", "-c", "am", "start",
                "-n", component, "-a", "android.intent.action.VIEW",
                "-t", "application/octet-stream",
                "-d", documentUri.toString(),
                "--grant-read-uri-permission")
        }

        try {
            val proc = Runtime.getRuntime().exec(amCmd)
            val exitCode = proc.waitFor()
            if (exitCode != 0) {
                isGameRunning = false
                throw Exception("Eden 模拟器未安装")
            }
            Log.d(TAG, "Launched Eden via root am start")
            // Root am start launches in separate task, move AppDeck to back
            if (displayId == null) {
                withContext(Dispatchers.Main) {
                    activity.moveTaskToBack(true)
                }
            }
            if (displayId != null) {
                startProcessMonitor(EDEN_PACKAGE)
            }
        } catch (e: Exception) {
            isGameRunning = false
            throw Exception("Eden 启动失败: ${e.message}")
        }
    }

    /** Called by MainActivity when SAF tree permission is granted */
    fun onSafTreeGranted(treeUri: Uri) {
        Log.d(TAG, "SAF tree granted: $treeUri")

        // Persist the tree URI
        activity.contentResolver.takePersistableUriPermission(
            treeUri, Intent.FLAG_GRANT_READ_URI_PERMISSION
        )
        val prefs = activity.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        prefs.edit().putString(PREF_SAF_TREE_URI, treeUri.toString()).apply()

        // Resume pending Eden launch
        val data = pendingEdenLaunch
        pendingEdenLaunch = null
        if (data != null) {
            scope.launch {
                try {
                    if (sessionEndedDuringLaunch) {
                        currentLaunchData = null; currentGamePackage = null
                        return@launch
                    }
                    launchEdenWithTreeUri(data, treeUri)
                } catch (e: Exception) {
                    Log.e(TAG, "Eden launch after SAF grant failed", e)
                    isGameRunning = false
                    currentLaunchData = null
                    currentGamePackage = null
                    notifyError(e.message ?: "启动失败")
                }
            }
        }
    }

    /** Called by MainActivity when SAF tree permission is denied */
    fun onSafTreeDenied() {
        Log.w(TAG, "SAF tree permission denied")
        pendingEdenLaunch = null
        currentLaunchData = null
        notifyError("需要授权游戏目录访问权限")
    }

    private fun getSavedTreeUri(): Uri? {
        val prefs = activity.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        val uriStr = prefs.getString(PREF_SAF_TREE_URI, null) ?: return null
        val uri = Uri.parse(uriStr)

        // Verify we still have permission
        val persistedUris = activity.contentResolver.persistedUriPermissions
        val hasPermission = persistedUris.any { it.uri == uri && it.isReadPermission }
        if (!hasPermission) {
            Log.w(TAG, "SAF tree permission expired, clearing saved URI")
            prefs.edit().remove(PREF_SAF_TREE_URI).apply()
            return null
        }

        return uri
    }

    private suspend fun launchAndroidApp(data: GameLaunchData) {
        if (sessionEndedDuringLaunch) {
            currentLaunchData = null; currentGamePackage = null
            return
        }

        // Support "package/activity" format or plain package name
        val parts = data.launchParam.split("/", limit = 2)
        val packageName = parts[0]
        val activityName = if (parts.size > 1) parts[1] else null

        isGameRunning = true
        currentGamePackage = packageName
        writeCurrentGamePackage(packageName)
        val displayId = externalDisplayId
        launchedOnExternalDisplay = displayId != null

        // Resolve component for am start (needed for --display flag)
        val component = if (activityName != null) {
            "$packageName/$activityName"
        } else {
            // Resolve launcher activity from package manager instead of using monkey
            val launchIntent = activity.packageManager.getLaunchIntentForPackage(packageName)
            launchIntent?.component?.flattenToShortString()
        }

        // Use root am start to bypass Android's intent confirmation dialog
        val amCmd = if (component != null && displayId != null) {
            arrayOf("su", "-c", "am", "start", "--display", displayId.toString(), "-n", component)
        } else if (component != null) {
            arrayOf("su", "-c", "am", "start", "-n", component)
        } else {
            arrayOf("su", "-c", "monkey", "-p", packageName, "-c", "android.intent.category.LAUNCHER", "1")
        }

        try {
            val proc = Runtime.getRuntime().exec(amCmd)
            val exitCode = proc.waitFor()
            if (exitCode != 0) {
                isGameRunning = false
                throw Exception("应用未安装: $packageName")
            }
            Log.d(TAG, "Launched $packageName via root am start")
            // Root am start doesn't trigger normal activity transition,
            // so explicitly move AppDeck to back so game appears in front
            if (displayId == null) {
                withContext(Dispatchers.Main) {
                    activity.moveTaskToBack(true)
                }
            }
            if (displayId != null) {
                startProcessMonitor(packageName)
            }
        } catch (e: Exception) {
            isGameRunning = false
            throw Exception("启动应用失败: $packageName - ${e.message}")
        }
    }

    fun onGameReturned() {
        if (!isGameRunning) return
        val wasOnExternalDisplay = launchedOnExternalDisplay
        isGameRunning = false
        // Notify server of app quit via WebSocket
        webSocket?.sendAppQuit("user_quit")
        launchedOnExternalDisplay = false
        processMonitorThread?.interrupt()
        processMonitorThread = null

        // Capture and clear state synchronously to avoid race with immediate relaunch
        val data = currentLaunchData
        val pkg = currentGamePackage
        currentLaunchData = null
        moonlightHostId = null
        currentGamePackage = null

        scope.launch {
            // Only force-stop if the process is still alive.
            // Skip if gamepad monitor already killed it — avoids killing
            // a newly launched instance of the same package.
            if (pkg != null && isProcessAlive(pkg)) {
                forceStopPackage(pkg)
            }

            // Release moonlight host if applicable
            if (data?.launchType == "moonlight" || (data?.launchType != "retroarch" && data?.launchType != "eden" && data?.launchType != "android")) {
                releaseMoonlightHost(data)
            }

            clearCurrentGamePackage()

            withContext(Dispatchers.Main) {
                if (wasOnExternalDisplay) {
                    activity.restoreExternalDisplay()
                }
                activity.evaluateJavascript("window.onGameExited && window.onGameExited({})")
            }
        }
    }

    fun onSessionEnded() {
        if (!isGameRunning) {
            sessionEndedDuringLaunch = true
            return
        }
        val wasOnExternalDisplay = launchedOnExternalDisplay
        isGameRunning = false
        // Notify server of app quit via WebSocket
        webSocket?.sendAppQuit("session_end")
        launchedOnExternalDisplay = false
        processMonitorThread?.interrupt()
        processMonitorThread = null

        // Capture and clear state synchronously to avoid race with immediate relaunch
        val data = currentLaunchData
        val pkg = currentGamePackage
        currentLaunchData = null
        moonlightHostId = null
        currentGamePackage = null

        scope.launch {
            if (data?.launchType == "moonlight" || (data?.launchType != "retroarch" && data?.launchType != "eden" && data?.launchType != "android")) {
                releaseMoonlightHost(data)
            }
            clearCurrentGamePackage()
        }

        // Restore presentation before killing game
        if (wasOnExternalDisplay) {
            activity.runOnUiThread { activity.restoreExternalDisplay() }
        }

        // Bring our activity to front and kill the game
        bringToFrontAndKillGame(pkg)
    }

    fun forceStopCurrentGame() {
        if (!isGameRunning) return
        isGameRunning = false
        // Notify server of app quit via WebSocket
        webSocket?.sendAppQuit("force_quit")
        val wasOnExternalDisplay = launchedOnExternalDisplay
        launchedOnExternalDisplay = false
        processMonitorThread?.interrupt()
        processMonitorThread = null

        // Capture and clear state synchronously so onGameReturned (via onResume) is a no-op
        // and immediate relaunch won't race with the cleanup coroutine
        val data = currentLaunchData
        val pkg = currentGamePackage
        currentLaunchData = null
        moonlightHostId = null
        currentGamePackage = null

        scope.launch {
            if (data?.launchType == "moonlight" || (data?.launchType != "retroarch" && data?.launchType != "eden" && data?.launchType != "android")) {
                releaseMoonlightHost(data)
            }
            clearCurrentGamePackage()
        }

        // Restore presentation before killing game
        if (wasOnExternalDisplay) {
            activity.runOnUiThread { activity.restoreExternalDisplay() }
        }

        // Bring our activity to front and kill the game
        bringToFrontAndKillGame(pkg)
    }

    private fun bringToFrontAndKillGame(pkg: String?) {
        activity.runOnUiThread {
            val intent = Intent(activity, MainActivity::class.java).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_REORDER_TO_FRONT)
            }
            activity.startActivity(intent)

            if (pkg != null) {
                Thread { forceStopPackage(pkg) }.start()
            }
        }
    }

    private fun isProcessAlive(pkg: String): Boolean {
        return try {
            val proc = Runtime.getRuntime().exec(arrayOf("su", "-c", "pidof $pkg"))
            proc.waitFor() == 0
        } catch (e: Exception) { false }
    }

    private fun forceStopPackage(pkg: String) {
        try {
            val proc = Runtime.getRuntime().exec(arrayOf("su", "-c", "am", "force-stop", pkg))
            val exitCode = proc.waitFor()
            if (exitCode == 0) {
                Log.d(TAG, "Force-stopped $pkg via root")
            } else {
                Log.w(TAG, "su am force-stop exited with $exitCode, falling back")
                fallbackKill(pkg)
            }
        } catch (e: Exception) {
            Log.w(TAG, "Root force-stop failed for $pkg, falling back", e)
            fallbackKill(pkg)
        }
    }

    private fun fallbackKill(pkg: String) {
        try {
            val am = activity.getSystemService(Context.ACTIVITY_SERVICE) as android.app.ActivityManager
            am.killBackgroundProcesses(pkg)
        } catch (e: Exception) {
            Log.e(TAG, "Fallback kill failed for $pkg", e)
        }
    }

    private fun releaseMoonlightHost(data: GameLaunchData?) {
        if (data == null) return
        try {
            val body = JSONObject().apply {
                put("device_id", "${data.hostId}:${data.machineCode}")
            }.toString().toRequestBody(jsonType)

            val request = Request.Builder()
                .url("$BASE_URL/client/moonlight/release")
                .post(body)
                .build()

            httpClient.newCall(request).execute()
            Log.d(TAG, "Moonlight host released")
        } catch (e: Exception) {
            Log.e(TAG, "Moonlight release error", e)
        }
    }

    private fun parseJson(jsonData: String): GameLaunchData? {
        return try {
            val json = JSONObject(jsonData)
            GameLaunchData(
                id = json.optString("id", ""),
                name = json.optString("name", ""),
                launchType = json.optString("launchType", "moonlight"),
                launchParam = json.optString("launchParam", ""),
                retroarchCore = json.optString("retroarchCore", ""),
                retroarchRom = json.optString("retroarchRom", ""),
                authToken = json.optString("authToken", ""),
                sessionId = json.optString("sessionId", ""),
                freePlay = json.optBoolean("freePlay", false),
                hostId = json.optString("hostId", DeviceIdentity.hostId),
                machineCode = json.optString("machineCode", DeviceIdentity.machineCode)
            )
        } catch (e: Exception) {
            Log.e(TAG, "Failed to parse launch data", e)
            null
        }
    }

    private fun notifyError(message: String) {
        Log.e(TAG, "Launch error: $message")
        activity.runOnUiThread {
            activity.evaluateJavascript("window.onGameLaunchError && window.onGameLaunchError('$message')")
        }
    }

    fun destroy() {
        scope.cancel()
    }
}
