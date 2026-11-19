package com.appdeck2.client

import android.content.Context
import android.provider.Settings
import android.util.Log
import java.util.UUID

object DeviceIdentity {
    private const val TAG = "DeviceIdentity"
    private const val PREFS_NAME = "device_identity"
    private const val KEY_MACHINE_CODE = "machine_code"

    private var _machineCode: String? = null

    fun init(context: Context) {
        // Try Settings.Secure.ANDROID_ID first
        val androidId = Settings.Secure.getString(context.contentResolver, Settings.Secure.ANDROID_ID)
        Log.d(TAG, "Settings.Secure.ANDROID_ID = $androidId")

        if (!androidId.isNullOrEmpty()) {
            _machineCode = androidId
            return
        }

        // Fallback: persistent UUID in SharedPreferences
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        var saved = prefs.getString(KEY_MACHINE_CODE, null)
        if (saved.isNullOrEmpty()) {
            saved = UUID.randomUUID().toString().replace("-", "").take(16)
            prefs.edit().putString(KEY_MACHINE_CODE, saved).apply()
            Log.d(TAG, "Generated fallback machine_code: $saved")
        } else {
            Log.d(TAG, "Loaded fallback machine_code: $saved")
        }
        _machineCode = saved
    }

    val hostId: String
        get() = (_machineCode ?: "unknown").take(8)

    val machineCode: String
        get() = _machineCode ?: "unknown"
}
