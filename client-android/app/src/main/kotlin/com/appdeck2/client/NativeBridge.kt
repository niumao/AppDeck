package com.appdeck2.client

import android.os.Handler
import android.os.Looper
import android.util.Log
import android.webkit.JavascriptInterface

interface NativeBridgeCallback {
    fun onLaunchGame(jsonData: String)
    fun onLogout()
    fun getHostId(): String
    fun getMachineCode(): String
    fun getDeviceState(): String?
    fun getScreensaverVideoUrl(videoId: String, videoHash: String): String
}

class NativeBridge(private val callback: NativeBridgeCallback) {

    private val mainHandler = Handler(Looper.getMainLooper())

    @JavascriptInterface
    fun launchGame(jsonData: String) {
        mainHandler.post {
            callback.onLaunchGame(jsonData)
        }
    }

    @JavascriptInterface
    fun logout() {
        mainHandler.post {
            callback.onLogout()
        }
    }

    @JavascriptInterface
    fun log(message: String) {
        Log.d("NativeBridge", message)
    }

    @JavascriptInterface
    fun getHostId(): String {
        return callback.getHostId()
    }

    @JavascriptInterface
    fun getMachineCode(): String {
        return callback.getMachineCode()
    }

    @JavascriptInterface
    fun getDeviceState(): String {
        return callback.getDeviceState() ?: "{}"
    }

    @JavascriptInterface
    fun getScreensaverVideoUrl(videoId: String, videoHash: String): String {
        return callback.getScreensaverVideoUrl(videoId, videoHash)
    }
}
