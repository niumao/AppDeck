package com.appdeck2.client

import android.app.Presentation
import android.content.Context
import android.graphics.Bitmap
import android.graphics.Color
import android.media.MediaPlayer
import android.net.Uri
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.Display
import android.view.InputDevice
import android.view.KeyEvent
import android.view.SurfaceHolder
import android.view.SurfaceView
import android.view.View
import android.view.WindowManager
import android.webkit.*
import android.widget.ImageView
import androidx.webkit.WebViewAssetLoader
import java.io.ByteArrayInputStream
import java.io.File
import java.io.FileInputStream
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class TvPresentation(
    context: Context,
    display: Display,
    private val callback: NativeBridgeCallback,
    private val preferredModeId: Int = 0
) : Presentation(context, display) {

    companion object {
        private const val TAG = "TvPresentation"
        private const val BASE_URL = "http://xiongmaovr.cn"
    }

    private val assetLoader by lazy {
        WebViewAssetLoader.Builder()
            .addPathHandler("/screensaver/",
                WebViewAssetLoader.InternalStoragePathHandler(context, File(context.filesDir, "screensaver")))
            .build()
    }

    lateinit var webView: WebView
        private set
    private lateinit var qrOverlay: ImageView
    private lateinit var screensaverSurface: SurfaceView

    // Screensaver state
    private var mediaPlayer: MediaPlayer? = null
    private var screensaverActive = false
    private var screensaverConfig: ScreensaverConfig? = null
    private val idleHandler = Handler(Looper.getMainLooper())
    private val idleRunnable = Runnable { showScreensaver() }
    private var surfaceReady = false
    private var pendingVideoPath: String? = null

    // SELECT handled by GamepadQuitMonitor daemon (3s hold → volume panel; hold + D-pad up/down → volume step)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Prevent system BACK key from dismissing the presentation
        setCancelable(false)

        window?.let { w ->
            // Fullscreen on external display
            @Suppress("DEPRECATION")
            w.setFlags(
                WindowManager.LayoutParams.FLAG_FULLSCREEN,
                WindowManager.LayoutParams.FLAG_FULLSCREEN
            )
            // Set 1080p mode if available
            if (preferredModeId != 0) {
                val attrs = w.attributes
                attrs.preferredDisplayModeId = preferredModeId
                w.attributes = attrs
                Log.d(TAG, "Set preferred display mode: $preferredModeId")
            }
        }

        setContentView(R.layout.tv_presentation)

        webView = findViewById(R.id.tvWebView)
        qrOverlay = findViewById(R.id.tvQrOverlay)
        screensaverSurface = findViewById(R.id.screensaverSurface)

        setupSurfaceHolder()
        setupWebView()
    }

    private fun setupSurfaceHolder() {
        screensaverSurface.holder.addCallback(object : SurfaceHolder.Callback {
            override fun surfaceCreated(holder: SurfaceHolder) {
                surfaceReady = true
                // Surface is now ready — start the pending MediaPlayer
                pendingVideoPath?.let { path ->
                    pendingVideoPath = null
                    startMediaPlayer(path, holder)
                }
            }

            override fun surfaceChanged(holder: SurfaceHolder, format: Int, width: Int, height: Int) {}

            override fun surfaceDestroyed(holder: SurfaceHolder) {
                surfaceReady = false
            }
        })
    }

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

        webView.addJavascriptInterface(NativeBridge(callback), "NativeBridge")

        webView.webViewClient = object : WebViewClient() {
            private var mainFrameError = false

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
                            ByteArrayInputStream(stream.toByteArray())
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
                // RetroArch local thumbnails via playlist lookup
                // URL: retroarch://thumb/{dir}/{romFile}
                if (url.scheme == "retroarch" && url.host == "thumb") {
                    // TvPresentation doesn't have access to retroarchThumbMap
                    // Return empty response to trigger placeholder
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
                    Log.e(TAG, "TV WebView error: ${error.description}")
                    mainFrameError = true
                    view.postDelayed({
                        view.settings.cacheMode = WebSettings.LOAD_DEFAULT
                        loadAppPage()
                    }, 5000)
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
                    Log.d("TVWebView", "${it.message()} [${it.sourceId()}:${it.lineNumber()}]")
                }
                return true
            }
        }
    }

    // --- Screensaver Config & Idle Timer ---

    fun updateScreensaverConfig(config: ScreensaverConfig) {
        screensaverConfig = config
        resetIdleTimer()
    }

    fun resetIdleTimer() {
        idleHandler.removeCallbacks(idleRunnable)
        val config = screensaverConfig ?: return
        if (!config.enabled) return
        if (config.defaultVideoId.isEmpty() && config.tenantVideoId.isEmpty()) return
        val delayMs = config.timeoutMinutes * 60 * 1000L
        idleHandler.postDelayed(idleRunnable, delayMs)
    }

    private fun isTenantVideoActive(config: ScreensaverConfig): Boolean {
        if (config.tenantVideoId.isEmpty()) return false

        val now = Date()
        val dateFormat = SimpleDateFormat("yyyy-MM-dd", Locale.US)

        if (config.tenantStartDate.isNotEmpty()) {
            val start = dateFormat.parse(config.tenantStartDate) ?: return false
            if (now.before(start)) return false
        }
        if (config.tenantEndDate.isNotEmpty()) {
            val end = dateFormat.parse(config.tenantEndDate) ?: return false
            // End date is inclusive — check against end of day
            val endOfDay = Date(end.time + 24 * 60 * 60 * 1000 - 1)
            if (now.after(endOfDay)) return false
        }

        if (config.tenantStartTime.isNotEmpty() || config.tenantEndTime.isNotEmpty()) {
            val hhmm = SimpleDateFormat("HH:mm", Locale.US).format(now)
            val startT = if (config.tenantStartTime.isNotEmpty()) config.tenantStartTime else "00:00"
            val endT = if (config.tenantEndTime.isNotEmpty()) config.tenantEndTime else "23:59"
            if (hhmm < startT || hhmm > endT) return false
        }

        return true
    }

    private fun resolveVideoFile(config: ScreensaverConfig): File? {
        val screensaverDir = File(context.filesDir, "screensaver")
        val videoId: String
        val videoHash: String

        if (isTenantVideoActive(config)) {
            videoId = config.tenantVideoId
            videoHash = config.tenantVideoHash
        } else if (config.defaultVideoId.isNotEmpty()) {
            videoId = config.defaultVideoId
            videoHash = config.defaultVideoHash
        } else {
            return null
        }

        // Verify hash file
        val hashFile = File(screensaverDir, "$videoId.hash")
        if (!hashFile.exists() || hashFile.readText().trim() != videoHash) {
            Log.w(TAG, "Screensaver video not cached or hash mismatch: $videoId")
            return null
        }

        val mp4 = File(screensaverDir, "$videoId.mp4")
        val mkv = File(screensaverDir, "$videoId.mkv")
        return when {
            mp4.exists() -> mp4
            mkv.exists() -> mkv
            else -> {
                Log.w(TAG, "Screensaver video file not found: $videoId")
                null
            }
        }
    }

    private fun showScreensaver() {
        val config = screensaverConfig ?: return
        if (screensaverActive || !config.enabled) return

        val videoFile = resolveVideoFile(config) ?: return

        screensaverActive = true
        screensaverSurface.visibility = View.VISIBLE

        if (surfaceReady) {
            // Surface already exists — start immediately
            startMediaPlayer(videoFile.absolutePath, screensaverSurface.holder)
        } else {
            // Surface not ready yet — defer to surfaceCreated callback
            pendingVideoPath = videoFile.absolutePath
        }
    }

    private fun startMediaPlayer(path: String, holder: SurfaceHolder) {
        if (!screensaverActive) return
        try {
            val player = MediaPlayer()
            player.setDisplay(holder)
            player.setDataSource(path)
            player.isLooping = true
            player.setOnPreparedListener { mp ->
                if (!screensaverActive) {
                    mp.release()
                    return@setOnPreparedListener
                }
                mp.start()
                Log.d(TAG, "Screensaver started: $path")
            }
            player.setOnErrorListener { _, what, extra ->
                Log.e(TAG, "Screensaver MediaPlayer error: what=$what extra=$extra")
                hideScreensaver()
                true
            }
            player.prepareAsync()
            mediaPlayer = player
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start screensaver", e)
            screensaverActive = false
            screensaverSurface.visibility = View.GONE
        }
    }

    fun hideScreensaver() {
        if (!screensaverActive) return
        screensaverActive = false
        pendingVideoPath = null

        val mp = mediaPlayer
        mediaPlayer = null
        mp?.let {
            try {
                it.stop()
            } catch (_: Exception) {}
            try {
                it.release()
            } catch (_: Exception) {}
        }

        screensaverSurface.visibility = View.GONE
        resetIdleTimer()
    }

    fun resetScreensaver() {
        if (screensaverActive) {
            hideScreensaver()
        } else {
            resetIdleTimer()
        }
    }

    override fun onStop() {
        // Clean up screensaver resources
        idleHandler.removeCallbacks(idleRunnable)
        mediaPlayer?.let { mp ->
            try {
                if (mp.isPlaying) mp.stop()
                mp.release()
            } catch (_: Exception) {}
        }
        mediaPlayer = null
        screensaverActive = false
        super.onStop()
    }

    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        // Handle gamepad events directly on the TV WebView
        val sourceClass = event.source and InputDevice.SOURCE_CLASS_MASK
        if (sourceClass == InputDevice.SOURCE_CLASS_BUTTON ||
            sourceClass == InputDevice.SOURCE_CLASS_JOYSTICK) {

            // Screensaver interception: any gamepad ACTION_DOWN dismisses it
            if (event.action == KeyEvent.ACTION_DOWN && screensaverActive) {
                hideScreensaver()
                return true
            }

            // Reset idle timer on any gamepad input
            if (event.action == KeyEvent.ACTION_DOWN) {
                resetIdleTimer()
            }

            // SELECT is handled by GamepadQuitMonitor daemon (3s hold → volume panel; hold + D-pad up/down → volume step)

            if (event.action != KeyEvent.ACTION_DOWN) {
                // Consume ACTION_UP for handled keys
                return true
            }

            when (event.keyCode) {
                KeyEvent.KEYCODE_DPAD_UP -> {
                    webView.evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('up')", null)
                    return true
                }
                KeyEvent.KEYCODE_DPAD_DOWN -> {
                    webView.evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('down')", null)
                    return true
                }
                KeyEvent.KEYCODE_DPAD_LEFT -> {
                    webView.evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('left')", null)
                    return true
                }
                KeyEvent.KEYCODE_DPAD_RIGHT -> {
                    webView.evaluateJavascript("window.onGamepadDirection && window.onGamepadDirection('right')", null)
                    return true
                }
                KeyEvent.KEYCODE_BUTTON_A,
                KeyEvent.KEYCODE_ENTER,
                KeyEvent.KEYCODE_DPAD_CENTER -> {
                    webView.evaluateJavascript("window.onGamepadButton && window.onGamepadButton('a')", null)
                    return true
                }
                KeyEvent.KEYCODE_BUTTON_B,
                KeyEvent.KEYCODE_BACK -> {
                    webView.evaluateJavascript("window.onGamepadButton && window.onGamepadButton('b')", null)
                    return true
                }
                KeyEvent.KEYCODE_BUTTON_L1 -> {
                    webView.evaluateJavascript("window.onGamepadButton && window.onGamepadButton('l1')", null)
                    return true
                }
                KeyEvent.KEYCODE_BUTTON_R1 -> {
                    webView.evaluateJavascript("window.onGamepadButton && window.onGamepadButton('r1')", null)
                    return true
                }
            }
        }
        return super.dispatchKeyEvent(event)
    }

    fun loadAppPage() {
        val hostId = callback.getHostId()
        val machineCode = callback.getMachineCode()
        val url = "$BASE_URL/app/?host_id=$hostId&machine_code=$machineCode&tv=1"
        Log.d(TAG, "TV loading $url")
        webView.loadUrl(url)
    }

    fun evaluateJavascript(script: String) {
        webView.evaluateJavascript(script, null)
    }

    fun setQrBitmap(bitmap: Bitmap?) {
        qrOverlay.setImageBitmap(bitmap)
    }

    fun showQrOverlay() {
        qrOverlay.visibility = View.VISIBLE
    }

    fun hideQrOverlay() {
        qrOverlay.visibility = View.GONE
    }
}
