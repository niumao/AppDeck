package com.appdeck2.client

import android.content.Context
import android.graphics.Bitmap
import android.hardware.display.DisplayManager
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.Display

class DisplayRouter(
    private val context: Context,
    private val callback: NativeBridgeCallback,
    private val onModeChanged: (DisplayMode) -> Unit
) {

    companion object {
        private const val TAG = "DisplayRouter"
    }

    enum class DisplayMode { SINGLE, DUAL }

    var mode = DisplayMode.SINGLE
        private set

    var tvPresentation: TvPresentation? = null
        private set

    private var cachedQrBitmap: Bitmap? = null
    private var qrVisible = false
    private var hiddenForGame = false
    private var cachedScreensaverConfig: ScreensaverConfig? = null

    private val displayManager = context.getSystemService(Context.DISPLAY_SERVICE) as DisplayManager
    private val mainHandler = Handler(Looper.getMainLooper())

    private val displayListener = object : DisplayManager.DisplayListener {
        override fun onDisplayAdded(displayId: Int) {
            Log.d(TAG, "Display added: $displayId")
            mainHandler.post { checkForExternalDisplay() }
        }

        override fun onDisplayRemoved(displayId: Int) {
            Log.d(TAG, "Display removed: $displayId")
            mainHandler.post { checkForExternalDisplay() }
        }

        override fun onDisplayChanged(displayId: Int) {
            // Re-check: display may have transitioned from OFF to ON after service tap
            mainHandler.post { checkForExternalDisplay() }
        }
    }

    fun start() {
        displayManager.registerDisplayListener(displayListener, mainHandler)
        checkForExternalDisplay()
    }

    fun stop() {
        displayManager.unregisterDisplayListener(displayListener)
        dismissPresentation()
    }

    private fun checkForExternalDisplay() {
        if (hiddenForGame) return // Don't interfere while game is on external display
        val presentationDisplays = displayManager.getDisplays(DisplayManager.DISPLAY_CATEGORY_PRESENTATION)
        val externalDisplay = presentationDisplays.firstOrNull()

        if (externalDisplay != null && mode == DisplayMode.SINGLE) {
            switchToDual(externalDisplay)
        } else if (externalDisplay == null && mode == DisplayMode.DUAL) {
            switchToSingle()
        }
    }

    private fun switchToDual(display: Display) {
        Log.d(TAG, "Switching to DUAL mode (display: ${display.name})")
        mode = DisplayMode.DUAL

        val preferredModeId = find1080pMode(display)
        val presentation = TvPresentation(context, display, callback, preferredModeId)

        // Re-show presentation if system dismisses it unexpectedly
        presentation.setOnDismissListener {
            Log.w(TAG, "Presentation dismissed unexpectedly, hiddenForGame=$hiddenForGame")
            if (!hiddenForGame && mode == DisplayMode.DUAL) {
                tvPresentation = null
                mode = DisplayMode.SINGLE  // Reset so checkForExternalDisplay can re-create
                mainHandler.post { checkForExternalDisplay() }
            }
        }

        presentation.show()
        presentation.loadAppPage()

        // Transfer cached QR
        cachedQrBitmap?.let { presentation.setQrBitmap(it) }
        if (!qrVisible) {
            presentation.hideQrOverlay()
        }

        tvPresentation = presentation
        cachedScreensaverConfig?.let { presentation.updateScreensaverConfig(it) }
        onModeChanged(DisplayMode.DUAL)
    }

    private fun switchToSingle() {
        Log.d(TAG, "Switching to SINGLE mode")
        mode = DisplayMode.SINGLE
        dismissPresentation()
        onModeChanged(DisplayMode.SINGLE)
    }

    private fun dismissPresentation() {
        tvPresentation?.let {
            try {
                it.dismiss()
            } catch (e: Exception) {
                Log.w(TAG, "Error dismissing presentation", e)
            }
        }
        tvPresentation = null
    }

    private fun find1080pMode(display: Display): Int {
        val modes = display.supportedModes
        for (mode in modes) {
            Log.d(TAG, "Supported mode: ${mode.modeId} ${mode.physicalWidth}x${mode.physicalHeight} @${mode.refreshRate}Hz")
            if (mode.physicalWidth == 1920 && mode.physicalHeight == 1080) {
                Log.d(TAG, "Found 1080p mode: ${mode.modeId}")
                return mode.modeId
            }
        }
        Log.d(TAG, "No 1080p mode found, using default")
        return 0
    }

    fun hideForGame() {
        if (mode != DisplayMode.DUAL) return
        Log.w(TAG, "Dismissing presentation for game on external display")
        tvPresentation?.hideScreensaver()
        hiddenForGame = true
        dismissPresentation()
    }

    fun showAfterGame() {
        if (!hiddenForGame) return
        hiddenForGame = false
        Log.w(TAG, "Recreating presentation after game exit")
        val displays = displayManager.getDisplays(DisplayManager.DISPLAY_CATEGORY_PRESENTATION)
        val display = displays.firstOrNull() ?: return
        val preferredModeId = find1080pMode(display)
        val presentation = TvPresentation(context, display, callback, preferredModeId)
        presentation.show()
        presentation.loadAppPage()
        cachedQrBitmap?.let { presentation.setQrBitmap(it) }
        if (!qrVisible) {
            presentation.hideQrOverlay()
        }
        tvPresentation = presentation
        cachedScreensaverConfig?.let { presentation.updateScreensaverConfig(it) }
    }

    fun getExternalDisplayId(): Int? {
        if (mode != DisplayMode.DUAL) return null
        val displays = displayManager.getDisplays(DisplayManager.DISPLAY_CATEGORY_PRESENTATION)
        return displays.firstOrNull()?.displayId
    }

    fun evaluateJavascript(script: String) {
        if (mode == DisplayMode.DUAL) {
            tvPresentation?.evaluateJavascript(script)
        } else {
            // Caller (MainActivity) handles SINGLE mode directly
        }
    }

    fun showQrOverlay(bitmap: Bitmap?) {
        cachedQrBitmap = bitmap
        qrVisible = true
        if (mode == DisplayMode.DUAL) {
            tvPresentation?.setQrBitmap(bitmap)
            tvPresentation?.showQrOverlay()
        }
    }

    fun hideQrOverlay() {
        qrVisible = false
        if (mode == DisplayMode.DUAL) {
            tvPresentation?.hideQrOverlay()
        }
    }

    fun resetScreensaver() {
        if (mode == DisplayMode.DUAL) {
            tvPresentation?.resetScreensaver()
        }
        // In SINGLE mode, MainActivity's own WebView handles this
    }

    fun updateScreensaverConfig(config: ScreensaverConfig) {
        cachedScreensaverConfig = config
        tvPresentation?.updateScreensaverConfig(config)
    }
}
