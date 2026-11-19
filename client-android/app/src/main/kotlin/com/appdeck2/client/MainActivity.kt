package com.appdeck2.client

import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.ServiceConnection
import android.hardware.display.DisplayManager
import android.graphics.Bitmap
import android.net.Uri
import android.os.Bundle
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.graphics.Color
import android.graphics.PixelFormat
import android.provider.DocumentsContract
import android.util.Log
import android.view.InputDevice
import android.view.KeyEvent
import android.view.MotionEvent
import android.view.Gravity
import android.view.View
import android.view.WindowManager
import android.webkit.*
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.TextView
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import com.google.zxing.BarcodeFormat
import com.google.zxing.qrcode.QRCodeWriter
import org.json.JSONObject
import androidx.webkit.WebViewAssetLoader
import java.io.ByteArrayInputStream
import java.io.File
import java.io.FileInputStream

class MainActivity : AppCompatActivity(), NativeBridgeCallback {

    companion object {
        private const val TAG = "MainActivity"
        private const val BASE_URL = "http://xiongmaovr.cn"
        private const val RETROARCH_ROM_BASE = "/storage/emulated/0/RetroArch/gam/"
        private const val RETROARCH_THUMBNAILS_DIR = "/storage/emulated/0/RetroArch/thumbnails"
        private const val RETROARCH_PLAYLISTS_DIR = "/storage/emulated/0/RetroArch/playlists"
    }

    private lateinit var webView: WebView
    private lateinit var qrOverlay: ImageView
    private lateinit var phoneController: PhoneControllerView
    private lateinit var gameLauncher: GameLauncher
    private lateinit var displayRouter: DisplayRouter

    private val handler = Handler(Looper.getMainLooper())

    private var isDeviceApproved = false
    private var lastQrBitmap: Bitmap? = null
    private var lastAnalogTime = 0L

    // SELECT+START combo for WebView cache refresh
    private var selectHeld = false
    private var startHeld = false

    // Recharge QR overlay (low balance warning)
    private var rechargeQrView: View? = null
    private var rechargeQrWm: WindowManager? = null
    private var rechargeCountdownTv: TextView? = null
    private var rechargeCountdownTimer: java.util.Timer? = null

    // Test overlay
    private var testOverlayView: TextView? = null

    private val testOverlayReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            when (intent.action) {
                "com.appdeck2.TEST_OVERLAY" -> showTestOverlay()
                "com.appdeck2.DISMISS_OVERLAY" -> dismissTestOverlay()
            }
        }
    }

    // Downloads and caches screensaver videos in internal storage
    private val screensaverVideoCache by lazy { ScreensaverVideoCache(filesDir) }

    // WebViewAssetLoader serves screensaver videos from internal storage
    private val assetLoader by lazy {
        WebViewAssetLoader.Builder()
            .addPathHandler("/screensaver/",
                WebViewAssetLoader.InternalStoragePathHandler(this, screensaverVideoCache.cacheDir))
            .build()
    }

    // RetroArch playlist entry: system name, game label, and core name from .lpl
    data class PlaylistEntry(val system: String, val label: String, val coreName: String)

    // RetroArch playlist map: relative ROM path → PlaylistEntry
    private val retroarchThumbMap = mutableMapOf<String, PlaylistEntry>()

    // SAF tree permission launcher for Eden games directory
    private val safTreeLauncher = registerForActivityResult(
        ActivityResultContracts.OpenDocumentTree()
    ) { uri: Uri? ->
        if (uri != null) {
            Log.d(TAG, "SAF tree granted: $uri")
            gameLauncher.onSafTreeGranted(uri)
        } else {
            Log.w(TAG, "SAF tree denied by user")
            gameLauncher.onSafTreeDenied()
        }
    }

    // --- WebSocket Service Binding ---

    private var appWebSocket: AppWebSocket? = null
    private var serviceBound = false

    private val serviceConnection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName, service: IBinder) {
            val binder = service as AppWebSocket.WebSocketBinder
            appWebSocket = binder.getService()
            appWebSocket?.callback = wsCallback
            gameLauncher.webSocket = appWebSocket
            serviceBound = true

            // Recover state if service was already connected
            appWebSocket?.getDeviceState()?.let { stateJson ->
                handleWsEvent("device_state_change", stateJson)
            }
        }

        override fun onServiceDisconnected(name: ComponentName) {
            appWebSocket?.callback = null
            appWebSocket = null
            gameLauncher.webSocket = null
            serviceBound = false
        }
    }

    private val wsCallback = object : WebSocketCallback {
        override fun onWsEvent(type: String, data: String) {
            handleWsEvent(type, data)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        DeviceIdentity.init(this)
        SaveSyncManager.retryIncompleteUploads(this, DeviceIdentity.hostId, DeviceIdentity.machineCode)
        gameLauncher = GameLauncher(this)
        parseRetroArchPlaylists()
        disableFreeformWindowing()
        fixEdenGamepadMapping()

        webView = findViewById(R.id.webView)
        qrOverlay = findViewById(R.id.qrOverlay)
        phoneController = findViewById(R.id.phoneController)

        displayRouter = DisplayRouter(this, this) { mode ->
            onDisplayModeChanged(mode)
        }

        setupKioskMode()
        setupWebView()

        // Start display auto-enable service (handles OxygenOS DP dialog)
        startForegroundService(Intent(this, DisplayAutoEnableService::class.java))

        // Start and bind to WebSocket service
        val wsIntent = Intent(this, AppWebSocket::class.java)
        startForegroundService(wsIntent)
        bindService(wsIntent, serviceConnection, Context.BIND_AUTO_CREATE)

        // Register test overlay broadcast receiver
        val overlayFilter = IntentFilter().apply {
            addAction("com.appdeck2.TEST_OVERLAY")
            addAction("com.appdeck2.DISMISS_OVERLAY")
        }
        registerReceiver(testOverlayReceiver, overlayFilter, Context.RECEIVER_EXPORTED)

        // Show pending screen until WS connects and sends initial state
        showPendingApproval()
    }

    override fun onResume() {
        super.onResume()
        reassertKioskMode()
        if (!gameLauncher.launchedOnExternalDisplay && gameLauncher.isGameRunning) {
            val pkg = gameLauncher.currentGamePackage
            if (pkg != null) {
                // Check if game process is still alive before acting.
                // During combo quit → relaunch, AppDeck may briefly gain focus while
                // the new game is starting. If the process is alive, yield back to it.
                val alive = try {
                    Runtime.getRuntime().exec(arrayOf("su", "-c", "pidof $pkg")).waitFor() == 0
                } catch (_: Exception) { false }
                if (alive) {
                    moveTaskToBack(true)
                    return
                }
            }
            gameLauncher.onGameReturned()
        }
    }

    override fun onDestroy() {
        hideRechargeQr()
        dismissTestOverlay()
        try { unregisterReceiver(testOverlayReceiver) } catch (_: Exception) {}
        if (serviceBound) {
            appWebSocket?.callback = null
            unbindService(serviceConnection)
            serviceBound = false
        }
        displayRouter.stop()
        gameLauncher.destroy()
        screensaverVideoCache.cancel()
        super.onDestroy()
    }

    override fun dispatchTouchEvent(ev: MotionEvent?): Boolean {
        if (ev?.action == MotionEvent.ACTION_DOWN) {
            if (displayRouter.mode == DisplayRouter.DisplayMode.DUAL) {
                displayRouter.resetScreensaver()
            }
        }
        return super.dispatchTouchEvent(ev)
    }

    override fun dispatchKeyEvent(event: KeyEvent?): Boolean {
        if (event == null) return super.dispatchKeyEvent(event)

        // Check if this is a gamepad event
        val sourceClass = event.source and InputDevice.SOURCE_CLASS_MASK
        if (sourceClass != InputDevice.SOURCE_CLASS_BUTTON &&
            sourceClass != InputDevice.SOURCE_CLASS_JOYSTICK) {
            return super.dispatchKeyEvent(event)
        }

        // Track SELECT/START for combo detection on both DOWN and UP
        when (event.keyCode) {
            KeyEvent.KEYCODE_BUTTON_SELECT -> {
                selectHeld = event.action == KeyEvent.ACTION_DOWN
                if (selectHeld && startHeld) {
                    refreshWebViewCache()
                    selectHeld = false
                    startHeld = false
                }
                return true
            }
            KeyEvent.KEYCODE_BUTTON_START -> {
                startHeld = event.action == KeyEvent.ACTION_DOWN
                if (selectHeld && startHeld) {
                    refreshWebViewCache()
                    selectHeld = false
                    startHeld = false
                }
                return true
            }
        }

        // In DUAL mode, dismiss or reset native TV screensaver on any gamepad input
        if (event.action == KeyEvent.ACTION_DOWN && displayRouter.mode == DisplayRouter.DisplayMode.DUAL) {
            displayRouter.resetScreensaver()
        }

        // Consume ACTION_UP for handled gamepad keys to prevent system defaults
        if (event.action != KeyEvent.ACTION_DOWN) {
            return when (event.keyCode) {
                KeyEvent.KEYCODE_DPAD_UP, KeyEvent.KEYCODE_DPAD_DOWN,
                KeyEvent.KEYCODE_DPAD_LEFT, KeyEvent.KEYCODE_DPAD_RIGHT,
                KeyEvent.KEYCODE_BUTTON_A, KeyEvent.KEYCODE_ENTER, KeyEvent.KEYCODE_DPAD_CENTER,
                KeyEvent.KEYCODE_BUTTON_B, KeyEvent.KEYCODE_BACK,
                KeyEvent.KEYCODE_BUTTON_L1, KeyEvent.KEYCODE_BUTTON_R1 -> true
                else -> super.dispatchKeyEvent(event)
            }
        }

        when (event.keyCode) {
            KeyEvent.KEYCODE_DPAD_UP -> {
                evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('up')")
                return true
            }
            KeyEvent.KEYCODE_DPAD_DOWN -> {
                evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('down')")
                return true
            }
            KeyEvent.KEYCODE_DPAD_LEFT -> {
                evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('left')")
                return true
            }
            KeyEvent.KEYCODE_DPAD_RIGHT -> {
                evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('right')")
                return true
            }
            KeyEvent.KEYCODE_BUTTON_A,
            KeyEvent.KEYCODE_ENTER,
            KeyEvent.KEYCODE_DPAD_CENTER -> {
                evaluateJavascript("window.onGamepadButton && window.onGamepadButton('a')")
                return true
            }
            KeyEvent.KEYCODE_BUTTON_B,
            KeyEvent.KEYCODE_BACK -> {
                evaluateJavascript("window.onGamepadButton && window.onGamepadButton('b')")
                return true
            }
            KeyEvent.KEYCODE_BUTTON_L1 -> {
                evaluateJavascript("window.onGamepadButton && window.onGamepadButton('l1')")
                return true
            }
            KeyEvent.KEYCODE_BUTTON_R1 -> {
                evaluateJavascript("window.onGamepadButton && window.onGamepadButton('r1')")
                return true
            }
        }

        return super.dispatchKeyEvent(event)
    }

    override fun onGenericMotionEvent(event: MotionEvent?): Boolean {
        if (event == null) return super.onGenericMotionEvent(event)

        if ((event.source and InputDevice.SOURCE_JOYSTICK) != InputDevice.SOURCE_JOYSTICK) {
            return super.onGenericMotionEvent(event)
        }

        val now = System.currentTimeMillis()
        if (now - lastAnalogTime < 150) {
            return true
        }

        var x = event.getAxisValue(MotionEvent.AXIS_HAT_X)
        var y = event.getAxisValue(MotionEvent.AXIS_HAT_Y)

        if (x == 0f && y == 0f) {
            x = event.getAxisValue(MotionEvent.AXIS_X)
            y = event.getAxisValue(MotionEvent.AXIS_Y)
        }

        val deadzone = 0.5f
        var handled = false

        if (Math.abs(x) > deadzone) {
            val direction = if (x > 0) "right" else "left"
            evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('$direction')")
            lastAnalogTime = now
            handled = true
        } else if (Math.abs(y) > deadzone) {
            val direction = if (y > 0) "down" else "up"
            evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('$direction')")
            lastAnalogTime = now
            handled = true
        }

        return if (handled) true else super.onGenericMotionEvent(event)
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        // No-op: kiosk mode
    }

    // --- NativeBridgeCallback ---

    override fun onLaunchGame(jsonData: String) {
        handleGameLaunch(jsonData)
    }

    override fun onLogout() {
        val message = JSONObject().apply {
            put("type", "logout")
        }.toString()
        sendWsMessage(message)
    }

    override fun getHostId(): String = DeviceIdentity.hostId

    override fun getMachineCode(): String = DeviceIdentity.machineCode

    override fun getDeviceState(): String? {
        return appWebSocket?.getDeviceState()
    }

    override fun getScreensaverVideoUrl(videoId: String, videoHash: String): String {
        return screensaverVideoCache.playableUrlOrEmpty(videoId, videoHash)
    }

    // --- Display Mode ---

    private fun onDisplayModeChanged(mode: DisplayRouter.DisplayMode) {
        when (mode) {
            DisplayRouter.DisplayMode.DUAL -> {
                webView.loadUrl("about:blank")  // Kill all JS & media (page reloaded on SINGLE switch)
                webView.visibility = View.GONE
                qrOverlay.visibility = View.GONE
                phoneController.visibility = View.GONE
                window.decorView.setBackgroundColor(Color.BLACK)
                window.attributes = window.attributes.apply {
                    screenBrightness = 0f
                }
                Log.d(TAG, "DUAL mode: phone screen blacked out, external display active")
            }
            DisplayRouter.DisplayMode.SINGLE -> {
                phoneController.visibility = View.GONE
                phoneController.tvWebView = null
                window.decorView.setBackgroundColor(Color.parseColor("#0a1628"))
                window.attributes = window.attributes.apply {
                    screenBrightness = WindowManager.LayoutParams.BRIGHTNESS_OVERRIDE_NONE
                }
                webView.visibility = View.VISIBLE
                loadAppPage()
                // Re-show QR from cached state if needed
                appWebSocket?.getDeviceState()?.let { stateJson ->
                    try {
                        val stateObj = JSONObject(stateJson)
                        val qrUrl = stateObj.optString("qr_url", "")
                        if (!isFreePlay && qrUrl.isNotEmpty()) {
                            showQrFromUrl(qrUrl)
                        }
                    } catch (e: Exception) {}
                }
                Log.d(TAG, "SINGLE mode: phone screen restored")
            }
        }
    }

    // --- Kiosk Mode ---

    private fun setupKioskMode() {
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        reassertKioskMode()
    }

    private fun reassertKioskMode() {
        @Suppress("DEPRECATION")
        window.decorView.systemUiVisibility = (
            View.SYSTEM_UI_FLAG_FULLSCREEN or
            View.SYSTEM_UI_FLAG_HIDE_NAVIGATION or
            View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
        )
    }

    // --- WebView ---

    private var mainFrameError = false

    private fun setupWebView() {
        webView.setBackgroundColor(Color.parseColor("#0a1628"))

        webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            cacheMode = WebSettings.LOAD_DEFAULT
            mediaPlaybackRequiresUserGesture = false
            @Suppress("DEPRECATION")
            allowFileAccess = true
        }

        webView.addJavascriptInterface(NativeBridge(this), "NativeBridge")

        webView.webViewClient = object : WebViewClient() {
            override fun shouldInterceptRequest(
                view: WebView,
                request: WebResourceRequest
            ): WebResourceResponse? {
                val url = request.url
                if (url.scheme == "pkg") {
                    val packageName = url.host ?: return null
                    return try {
                        val icon = view.context.packageManager.getApplicationIcon(packageName)
                        val bitmap = if (icon is android.graphics.drawable.BitmapDrawable) {
                            icon.bitmap
                        } else {
                            val bmp = Bitmap.createBitmap(
                                icon.intrinsicWidth, icon.intrinsicHeight, Bitmap.Config.ARGB_8888
                            )
                            val canvas = android.graphics.Canvas(bmp)
                            icon.setBounds(0, 0, canvas.width, canvas.height)
                            icon.draw(canvas)
                            bmp
                        }
                        val stream = java.io.ByteArrayOutputStream()
                        bitmap.compress(Bitmap.CompressFormat.PNG, 100, stream)
                        WebResourceResponse(
                            "image/png", null,
                            java.io.ByteArrayInputStream(stream.toByteArray())
                        )
                    } catch (e: Exception) {
                        Log.w(TAG, "Failed to load icon for package: $packageName", e)
                        null
                    }
                }
                if (url.scheme == "eden") {
                    val xciName = url.lastPathSegment ?: return null
                    val iconFile = File("/storage/emulated/0/Documents/swiicons/$xciName.jpg")
                    return try {
                        if (iconFile.exists()) {
                            WebResourceResponse("image/jpeg", null, FileInputStream(iconFile))
                        } else {
                            null
                        }
                    } catch (e: Exception) {
                        Log.w(TAG, "Eden icon access denied: ${iconFile.name}", e)
                        null
                    }
                }
                if (url.scheme == "retroarch" && url.host == "thumb") {
                    val rawPath = url.path?.trimStart('/') ?: return WebResourceResponse(
                        "image/png", null, ByteArrayInputStream(ByteArray(0))
                    )
                    val romBase = RETROARCH_ROM_BASE.trimStart('/')
                    val romPath = if (rawPath.startsWith(romBase)) {
                        rawPath.removePrefix(romBase)
                    } else {
                        rawPath
                    }
                    val entry = retroarchThumbMap[romPath] ?: run {
                        val lookupName = romPath.substringAfterLast('/').replace(" ", "")
                        retroarchThumbMap.entries.firstOrNull { (k, _) ->
                            k.substringAfterLast('/').replace(" ", "") == lookupName
                        }?.value
                    }
                    if (entry != null) {
                        val system = entry.system
                        val label = entry.label
                        val sanitizedLabel = label.replace(Regex("[&*/:\"<>?\\\\|]"), "_")
                        val thumbFile = File("$RETROARCH_THUMBNAILS_DIR/$system/Named_Boxarts/$sanitizedLabel.png")
                        try {
                            if (thumbFile.exists()) {
                                return WebResourceResponse("image/png", null, FileInputStream(thumbFile))
                            }
                        } catch (e: Exception) {
                            Log.w(TAG, "RetroArch thumb access denied: ${thumbFile.name}", e)
                        }
                    }
                    return WebResourceResponse("image/png", null, ByteArrayInputStream(ByteArray(0)))
                }
                if (url.host == "appassets.androidplatform.net") {
                    return assetLoader.shouldInterceptRequest(request.url)
                }
                return super.shouldInterceptRequest(view, request)
            }

            override fun onReceivedError(
                view: WebView, request: WebResourceRequest, error: WebResourceError
            ) {
                if (request.isForMainFrame) {
                    Log.e(TAG, "WebView main frame error: ${error.description} (code=${error.errorCode})")
                    mainFrameError = true
                    handler.postDelayed({
                        Log.d(TAG, "Retrying page load after error...")
                        view.settings.cacheMode = WebSettings.LOAD_DEFAULT
                        loadAppPage()
                    }, 5000)
                } else {
                    Log.w(TAG, "WebView sub-resource error: ${request.url} - ${error.description}")
                }
            }

            override fun onPageFinished(view: WebView, url: String?) {
                super.onPageFinished(view, url)
                if (!mainFrameError) {
                    view.settings.cacheMode = WebSettings.LOAD_DEFAULT
                }
                mainFrameError = false
            }
        }

        webView.webChromeClient = object : WebChromeClient() {
            override fun onConsoleMessage(message: ConsoleMessage?): Boolean {
                message?.let {
                    Log.d("WebView", "${it.message()} [${it.sourceId()}:${it.lineNumber()}]")
                }
                return true
            }
        }
    }

    private fun loadAppPage() {
        val hostId = DeviceIdentity.hostId
        val machineCode = DeviceIdentity.machineCode
        val url = "$BASE_URL/app/?host_id=$hostId&machine_code=$machineCode"
        Log.d(TAG, "Loading $url")
        webView.loadUrl(url)
    }

    private fun refreshWebViewCache() {
        Log.d(TAG, "SELECT+START combo: refreshing WebView cache")
        runOnUiThread {
            webView.clearCache(true)
            webView.evaluateJavascript(
                "try{localStorage.removeItem('cache_apps_full');" +
                "localStorage.removeItem('cache_plans_promo');" +
                "localStorage.removeItem('cache_rankings_hot');" +
                "localStorage.removeItem('cache_rankings_trending');" +
                "localStorage.removeItem('cache_rankings_regional');}catch(e){}", null
            )
            loadAppPage()
        }
    }

    // --- Device State ---

    private var isFreePlay = false

    private fun onDeviceApproved() {
        if (isDeviceApproved) return // Already approved
        isDeviceApproved = true
        loadAppPage()
        displayRouter.start()
        extractEdenIcons()
    }

    private fun extractEdenIcons() {
        Thread {
            try {
                val script = "/data/local/tmp/extract_eden_icons.sh"
                val scriptFile = java.io.File(script)
                if (!scriptFile.exists()) {
                    Log.d(TAG, "Eden icon extraction script not found, skipping")
                    return@Thread
                }
                Log.d(TAG, "Starting Eden icon extraction...")
                val proc = Runtime.getRuntime().exec(arrayOf("su", "-c", script))
                proc.waitFor()
                Log.d(TAG, "Eden icon extraction finished (exit=${proc.exitValue()})")
            } catch (e: Exception) {
                Log.w(TAG, "Eden icon extraction failed", e)
            }
        }.start()
    }

    private fun showPendingApproval() {
        val html = "<html><head><meta charset='UTF-8'></head>" +
            "<body style='display:flex;justify-content:center;align-items:center;height:100vh;" +
            "background:#0a1628;color:white;font-family:sans-serif;font-size:24px;'>" +
            "设备待审核，请联系管理员</body></html>"
        webView.loadDataWithBaseURL(null, html, "text/html", "UTF-8", null)
    }

    private fun showOfflineMessage() {
        val html = "<html><head><meta charset='UTF-8'></head>" +
            "<body style='display:flex;justify-content:center;align-items:center;height:100vh;" +
            "background:#0a1628;color:white;font-family:sans-serif;font-size:24px;'>" +
            "无法连接服务器</body></html>"
        webView.loadDataWithBaseURL(null, html, "text/html", "UTF-8", null)
    }

    // --- WS Event Handling ---

    private fun handleWsEvent(type: String, data: String) {
        Log.d(TAG, "WS event: $type")
        when (type) {
            "device_state_change" -> handleDeviceStateChange(data)
            "client_quit", "gamepad_quit" -> {
                gameLauncher.forceStopCurrentGame()
                evaluateJavascript("window.onNativeEvent && window.onNativeEvent('client_quit', $data)")
            }
            "screensaver_config" -> {
                handleScreensaverConfig(data)
            }
            "low_balance" -> {
                val nowSec = System.currentTimeMillis() / 1000
                val json = try { JSONObject(data) } catch (_: Exception) { null }
                val serverTs = json?.optLong("server_ts", nowSec) ?: nowSec
                val baseSeconds = json?.optDouble("base_seconds", 180.0) ?: 180.0
                showRechargeQr(serverTs, baseSeconds)
            }
            "low_balance_clear" -> {
                hideRechargeQr()
            }
            "disconnect_timeout" -> {
                gameLauncher.forceStopCurrentGame()
                hideQrOverlay()
                showOfflineMessage()
            }
            "save_sync_status" -> {
                evaluateJavascript("window.onNativeEvent && window.onNativeEvent('save_sync_status', $data)")
            }
        }
    }

    // device_state_change carries an "event" (state transition) and/or a legacy
    // "state" field; every branch used to end with its own copy of the same
    // forward-to-WebView call, so that forward now happens once here instead.
    private fun handleDeviceStateChange(data: String) {
        val json = JSONObject(data)
        val event = json.optString("event", "")
        val state = json.optString("state", "")
        isFreePlay = json.optBoolean("free_play", isFreePlay)

        when (event) {
            "connect" -> {
                onDeviceApproved()
                hideRechargeQr()
                // Safety net: if server says idle but a game is still running,
                // session must have ended while we were disconnected
                if (gameLauncher.isGameRunning) {
                    gameLauncher.onSessionEnded()
                }
                val qrUrl = json.optString("qr_url", "")
                if (qrUrl.isNotEmpty()) {
                    cacheQrBitmap(qrUrl)
                    if (!isFreePlay) showQrFromUrl(qrUrl)
                }
            }
            "login" -> {
                hideQrOverlay()
            }
            "session_end" -> {
                gameLauncher.onSessionEnded()
                hideRechargeQr()
                if (!isFreePlay) {
                    val qrUrl = json.optString("qr_url", "")
                    if (qrUrl.isNotEmpty()) showQrFromUrl(qrUrl)
                }
            }
            "free_play_change" -> {
                if (isFreePlay) {
                    hideQrOverlay()
                } else {
                    val qrUrl = json.optString("qr_url", "")
                    if (qrUrl.isNotEmpty()) showQrFromUrl(qrUrl)
                }
            }
            "balance_correction" -> {
                // No additional side effect; the forward below is all this needs.
            }
            "state_sync" -> {
                onDeviceApproved()
                val qrUrlSync = json.optString("qr_url", "")
                if (qrUrlSync.isNotEmpty()) cacheQrBitmap(qrUrlSync)
                when (state) {
                    "logged_in" -> hideQrOverlay()
                    "idle" -> {
                        // Safety net: if server says idle but a game is still running,
                        // session ended while Activity was destroyed or WS was down
                        if (gameLauncher.isGameRunning) {
                            gameLauncher.onSessionEnded()
                        }
                        if (!isFreePlay) {
                            val qrUrl = json.optString("qr_url", "")
                            if (qrUrl.isNotEmpty()) showQrFromUrl(qrUrl)
                        }
                    }
                }
            }
            else -> {
                // Backward compat: handle old-style events without event field
                when (state) {
                    "logged_in" -> {
                        onDeviceApproved()
                        hideQrOverlay()
                    }
                    "idle", "need_recharge" -> {
                        onDeviceApproved()
                        if (!isFreePlay) {
                            val qrUrl = json.optString("qr_url", "")
                            if (qrUrl.isNotEmpty()) showQrFromUrl(qrUrl)
                        }
                    }
                }
            }
        }

        evaluateJavascript("window.onNativeEvent && window.onNativeEvent('device_state_change', $data)")
    }

    // --- QR Code from URL ---

    private fun showQrFromUrl(qrUrl: String) {
        // QR is now rendered by H5 WebView in the sidebar — native overlay not needed
    }

    private fun hideQrOverlay() {
        displayRouter.hideQrOverlay()
        qrOverlay.visibility = View.GONE
    }

    // --- Recharge QR Overlay (low balance) ---

    private fun cacheQrBitmap(qrUrl: String) {
        if (lastQrBitmap != null) return
        try {
            val h5Url = "${BASE_URL}/h5/?host_id=${DeviceIdentity.hostId}&machine_code=${DeviceIdentity.machineCode}"
            val size = 256
            val bitMatrix = QRCodeWriter().encode(h5Url, BarcodeFormat.QR_CODE, size, size)
            val bitmap = Bitmap.createBitmap(size, size, Bitmap.Config.ARGB_8888)
            for (x in 0 until size) {
                for (y in 0 until size) {
                    bitmap.setPixel(x, y, if (bitMatrix[x, y]) Color.BLACK else Color.WHITE)
                }
            }
            lastQrBitmap = bitmap
            Log.d(TAG, "Generated recharge QR locally for $h5Url")
        } catch (e: Exception) {
            Log.w(TAG, "Failed to generate QR bitmap", e)
        }
    }

    private fun showRechargeQr(serverTs: Long, baseSeconds: Double = 180.0) {
        if (rechargeQrView != null) return
        val bitmap = lastQrBitmap ?: return

        // Pick the right display: external (TV) in DUAL mode, phone in SINGLE
        val wm: WindowManager
        if (displayRouter.mode == DisplayRouter.DisplayMode.DUAL) {
            val dm = getSystemService(Context.DISPLAY_SERVICE) as DisplayManager
            val extDisplay = dm.getDisplays(DisplayManager.DISPLAY_CATEGORY_PRESENTATION).firstOrNull()
            wm = if (extDisplay != null) {
                createDisplayContext(extDisplay).getSystemService(WINDOW_SERVICE) as WindowManager
            } else {
                getSystemService(WINDOW_SERVICE) as WindowManager
            }
        } else {
            wm = getSystemService(WINDOW_SERVICE) as WindowManager
        }

        val density = resources.displayMetrics.density

        val container = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setBackgroundColor(0xAA000000.toInt())
            val pad = (9 * density).toInt()
            setPadding(pad, pad, pad, pad)
            gravity = Gravity.CENTER
        }

        val size = (45 * density).toInt()
        val iv = ImageView(this).apply {
            setImageBitmap(bitmap)
            layoutParams = LinearLayout.LayoutParams(size, size)
        }

        val tv = TextView(this).apply {
            text = "充值"
            textSize = 10f
            setTextColor(Color.WHITE)
            gravity = Gravity.CENTER
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { topMargin = (2 * density).toInt() }
        }

        val countdownTv = TextView(this).apply {
            textSize = 10f
            setTextColor(0xFFFFD700.toInt())
            gravity = Gravity.CENTER
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { topMargin = (2 * density).toInt() }
        }
        rechargeCountdownTv = countdownTv

        container.addView(iv)
        container.addView(tv)
        container.addView(countdownTv)

        val params = WindowManager.LayoutParams(
            WindowManager.LayoutParams.WRAP_CONTENT,
            WindowManager.LayoutParams.WRAP_CONTENT,
            WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY,
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE,
            PixelFormat.TRANSLUCENT
        ).apply {
            gravity = Gravity.BOTTOM or Gravity.END
            x = 0
            y = 0
        }

        try {
            wm.addView(container, params)
        } catch (e: Exception) {
            Log.w(TAG, "Failed to show recharge QR overlay (missing overlay permission?)", e)
            return
        }
        rechargeQrView = container
        rechargeQrWm = wm
        Log.d(TAG, "Recharge QR overlay shown at bottom-right")

        // Start countdown timer
        startRechargeCountdown(serverTs, baseSeconds)
    }

    private fun startRechargeCountdown(serverTs: Long, baseSeconds: Double) {
        stopRechargeCountdown()
        // Re-derive remaining time from (baseSeconds, serverTs) on every tick,
        // the same calculation the H5 right panel uses — instead of decrementing
        // a local counter, which drifts from the server-truth countdown.
        fun remainingNow(): Int {
            val elapsed = (System.currentTimeMillis() / 1000.0) - serverTs
            return maxOf(0, (baseSeconds - elapsed).toInt())
        }
        updateCountdownText(remainingNow())
        rechargeCountdownTimer = java.util.Timer().apply {
            scheduleAtFixedRate(object : java.util.TimerTask() {
                override fun run() {
                    val r = remainingNow()
                    if (r <= 0) {
                        cancel()
                        runOnUiThread { hideRechargeQr() }
                        return
                    }
                    runOnUiThread { updateCountdownText(r) }
                }
            }, 1000, 1000)
        }
    }

    private fun stopRechargeCountdown() {
        rechargeCountdownTimer?.cancel()
        rechargeCountdownTimer = null
    }

    private fun updateCountdownText(seconds: Int) {
        val mins = seconds / 60
        val secs = seconds % 60
        rechargeCountdownTv?.text = String.format("%d:%02d", mins, secs)
    }

    private fun hideRechargeQr() {
        stopRechargeCountdown()
        rechargeCountdownTv = null
        rechargeQrView?.let {
            try { rechargeQrWm?.removeView(it) } catch (_: Exception) {}
            rechargeQrView = null
            rechargeQrWm = null
            Log.d(TAG, "Recharge QR overlay hidden")
        }
    }

    // --- Screensaver Config & Video Download ---

    private fun handleScreensaverConfig(data: String) {
        try {
            val json = JSONObject(data)
            val enabled = json.optBoolean("enabled", false)
            val timeout = json.optInt("timeout", 5)
            Log.d(TAG, "Screensaver config: enabled=$enabled, timeout=$timeout")

            val activeVideoIds = mutableSetOf<String>()

            // Download default video if present
            val defaultVideoId = json.optString("default_video_id", "")
            val defaultVideoHash = json.optString("default_video_hash", "")
            if (defaultVideoId.isNotEmpty() && defaultVideoHash.isNotEmpty()) {
                activeVideoIds.add(defaultVideoId)
                screensaverVideoCache.download(defaultVideoId, defaultVideoHash)
            }

            // Download tenant-specific video if present
            val tenantVideoId = json.optString("tenant_video_id", "")
            val tenantVideoHash = json.optString("tenant_video_hash", "")
            if (tenantVideoId.isNotEmpty() && tenantVideoHash.isNotEmpty()) {
                activeVideoIds.add(tenantVideoId)
                screensaverVideoCache.download(tenantVideoId, tenantVideoHash)
            }

            // Remove videos no longer referenced by current config
            screensaverVideoCache.cleanupStale(activeVideoIds)

            // Build ScreensaverConfig and forward to DisplayRouter for native TV screensaver
            val config = ScreensaverConfig(
                enabled = enabled,
                timeoutMinutes = timeout,
                defaultVideoId = defaultVideoId,
                defaultVideoHash = defaultVideoHash,
                tenantVideoId = tenantVideoId,
                tenantVideoHash = tenantVideoHash,
                tenantStartDate = json.optString("start_date", ""),
                tenantEndDate = json.optString("end_date", ""),
                tenantStartTime = json.optString("start_time", ""),
                tenantEndTime = json.optString("end_time", "")
            )
            displayRouter.updateScreensaverConfig(config)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to handle screensaver config", e)
        }
    }

    // --- SAF Tree Permission ---

    fun requestSafTreePermission() {
        val initialUri = DocumentsContract.buildDocumentUri(
            "com.android.externalstorage.documents",
            "primary:Documents/swi"
        )
        Log.d(TAG, "Requesting SAF tree, initial URI: $initialUri")
        safTreeLauncher.launch(initialUri)
    }

    // --- Game Launch ---

    fun handleGameLaunch(jsonData: String) {
        val modifiedJson = try {
            val json = JSONObject(jsonData)
            val romPathRaw = json.optString("retroarchRom", "")
            val trimmedRaw = romPathRaw.trimStart('/')
            val romBase = RETROARCH_ROM_BASE.trimStart('/')
            val romPath = if (trimmedRaw.startsWith(romBase)) {
                trimmedRaw.removePrefix(romBase)
            } else {
                trimmedRaw
            }
            if (json.optString("launchType", "") == "retroarch" && romPath.isNotEmpty()) {
                val entry = retroarchThumbMap[romPath]
                if (entry != null && entry.coreName.isNotEmpty()) {
                    Log.d(TAG, "Overriding retroarchCore from playlist: ${entry.coreName}")
                    json.put("retroarchCore", entry.coreName)
                    json.toString()
                } else {
                    jsonData
                }
            } else {
                jsonData
            }
        } catch (e: Exception) {
            Log.w(TAG, "Failed to check playlist core override", e)
            jsonData
        }
        val extDisplayId = displayRouter.getExternalDisplayId()
        gameLauncher.externalDisplayId = extDisplayId
        if (extDisplayId != null) {
            displayRouter.hideForGame()
        }
        gameLauncher.launch(modifiedJson)
    }

    fun restoreExternalDisplay() {
        if (displayRouter.mode == DisplayRouter.DisplayMode.DUAL) {
            displayRouter.showAfterGame()
        }
    }

    private fun disableFreeformWindowing() {
        Thread {
            try {
                Runtime.getRuntime().exec(arrayOf("su", "-c",
                    "settings put global enable_freeform_support 0")).waitFor()
                Runtime.getRuntime().exec(arrayOf("su", "-c",
                    "settings put global force_resizable_activities 0")).waitFor()
                Log.d(TAG, "Disabled freeform windowing for fullscreen game launches")
            } catch (e: Exception) {
                Log.w(TAG, "Failed to disable freeform windowing", e)
            }
        }.start()
    }

    /**
     * Eden maps gamepads by Nintendo diamond position rather than by Xbox label, so an
     * Xbox pad's raw SDL codes for A/B (96/97) and X/Y (99/100) land on the wrong Eden
     * button. Checked once at AppDeck startup (not per game launch) since it's a one-time
     * device config fix, and it's a no-op once already corrected.
     */
    private fun fixEdenGamepadMapping() {
        Thread {
            try {
                val configPath = "/storage/emulated/0/Android/data/${GameLauncher.EDEN_PACKAGE}/files/config/config.ini"

                val catProc = Runtime.getRuntime().exec(arrayOf("su", "-c", "cat '$configPath'"))
                val original = catProc.inputStream.bufferedReader().readText()
                catProc.waitFor()
                if (original.isBlank()) {
                    Log.d(TAG, "Eden config.ini not found, skipping gamepad fix")
                    return@Thread
                }

                val lines = original.lines().toMutableList()
                var changed = false
                val playerIndices = Regex("""player_(\d+)_button_a=""").findAll(original)
                    .map { it.groupValues[1] }.toSet()

                for (idx in playerIndices) {
                    if (swapButtonPair(lines, "player_${idx}_button_a", "player_${idx}_button_b", "96", "97")) changed = true
                    if (swapButtonPair(lines, "player_${idx}_button_x", "player_${idx}_button_y", "99", "100")) changed = true
                }

                if (!changed) {
                    Log.d(TAG, "Eden gamepad mapping already correct")
                    return@Thread
                }

                // Preserve original file ownership so Eden can still read/write its own config
                val statProc = Runtime.getRuntime().exec(arrayOf("su", "-c", "stat -c '%u:%g' '$configPath'"))
                val owner = statProc.inputStream.bufferedReader().readText().trim()
                statProc.waitFor()

                val tempFile = File(filesDir, "eden_config_fix.ini")
                tempFile.writeText(lines.joinToString("\n"))

                val pushCmd = if (owner.isNotEmpty()) {
                    "cp '${tempFile.absolutePath}' '$configPath' && chown $owner '$configPath'"
                } else {
                    "cp '${tempFile.absolutePath}' '$configPath'"
                }
                val exitCode = Runtime.getRuntime().exec(arrayOf("su", "-c", pushCmd)).waitFor()
                tempFile.delete()

                Log.d(TAG, if (exitCode == 0) "Eden gamepad A/B/X/Y mapping fixed"
                           else "Failed to write fixed Eden config, exit=$exitCode")
            } catch (e: Exception) {
                Log.w(TAG, "Eden gamepad mapping fix failed", e)
            }
        }.start()
    }

    private fun swapButtonPair(
        lines: MutableList<String>,
        keyA: String,
        keyB: String,
        codeA: String,
        codeB: String
    ): Boolean {
        val idxA = lines.indexOfFirst { it.startsWith("$keyA=") }
        val idxB = lines.indexOfFirst { it.startsWith("$keyB=") }
        if (idxA < 0 || idxB < 0) return false

        val lineA = lines[idxA]
        val lineB = lines[idxB]
        if (!lineA.contains("button:$codeB,") || !lineB.contains("button:$codeA,")) return false

        lines[idxA] = lineA.replaceFirst("button:$codeB,", "button:$codeA,")
        lines[idxB] = lineB.replaceFirst("button:$codeA,", "button:$codeB,")
        return true
    }

    fun sendWsMessage(message: String) {
        appWebSocket?.send(message)
    }

    fun evaluateJavascript(script: String) {
        runOnUiThread {
            if (displayRouter.mode == DisplayRouter.DisplayMode.DUAL) {
                displayRouter.evaluateJavascript(script)
            } else {
                webView.evaluateJavascript(script, null)
            }
        }
    }

    // --- Test Overlay ---

    private fun showTestOverlay() {
        if (testOverlayView != null) return  // Already showing
        val wm = getSystemService(WINDOW_SERVICE) as WindowManager
        val tv = TextView(this).apply {
            text = "余额不足，扫码充值"
            textSize = 28f
            setTextColor(Color.WHITE)
            setBackgroundColor(0xCC000000.toInt())
            setPadding(32, 20, 32, 20)
        }
        val params = WindowManager.LayoutParams(
            WindowManager.LayoutParams.WRAP_CONTENT,
            WindowManager.LayoutParams.WRAP_CONTENT,
            WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY,
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE,
            PixelFormat.TRANSLUCENT
        ).apply {
            gravity = Gravity.TOP or Gravity.CENTER_HORIZONTAL
            y = 40
        }
        wm.addView(tv, params)
        testOverlayView = tv
        Log.d(TAG, "Test overlay shown")
    }

    private fun dismissTestOverlay() {
        testOverlayView?.let {
            val wm = getSystemService(WINDOW_SERVICE) as WindowManager
            try { wm.removeView(it) } catch (_: Exception) {}
            testOverlayView = null
            Log.d(TAG, "Test overlay dismissed")
        }
    }

    // --- RetroArch Playlist Thumbnail Lookup ---

    private fun parseRetroArchPlaylists() {
        val playlistDir = File(RETROARCH_PLAYLISTS_DIR)
        if (!playlistDir.exists()) {
            Log.d(TAG, "RetroArch playlists dir not found")
            return
        }

        val lplFiles = playlistDir.listFiles { f -> f.isFile && f.extension == "lpl" } ?: return

        for (file in lplFiles) {
            try {
                val json = JSONObject(file.readText())
                val items = json.optJSONArray("items") ?: continue
                val systemName = file.nameWithoutExtension

                for (i in 0 until items.length()) {
                    val item = items.getJSONObject(i)
                    val path = item.optString("path", "")
                    val label = item.optString("label", "")
                    if (path.isEmpty() || label.isEmpty()) continue

                    val corePath = item.optString("core_path", "")
                    val coreName = if (corePath.isNotEmpty() && corePath != "DETECT") {
                        corePath.substringAfterLast('/')
                            .removeSuffix("_libretro_android.so")
                    } else {
                        ""
                    }

                    val relPath = if (path.startsWith(RETROARCH_ROM_BASE)) {
                        path.removePrefix(RETROARCH_ROM_BASE)
                    } else {
                        path.substringAfterLast('/')
                    }
                    retroarchThumbMap[relPath] = PlaylistEntry(systemName, label, coreName)
                }
            } catch (e: Exception) {
                Log.w(TAG, "Failed to parse playlist: ${file.name}", e)
            }
        }
        Log.d(TAG, "Loaded ${retroarchThumbMap.size} RetroArch playlist entries for thumbnail lookup")
    }
}
