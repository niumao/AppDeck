package com.appdeck2.client

import android.util.Log
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.security.MessageDigest
import java.util.concurrent.TimeUnit

/**
 * Downloads and caches screensaver videos in internal storage:
 * - Hash-based cache check (skip if already cached)
 * - Resume support via HTTP Range header
 * - SHA256 hash verification after download
 * - Local-only playback (never streams from server)
 */
class ScreensaverVideoCache(filesDir: File) {
    companion object {
        private const val TAG = "ScreensaverVideoCache"
        private const val BASE_URL = "http://xiongmaovr.cn"
    }

    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    val cacheDir: File = File(filesDir, "screensaver").also { it.mkdirs() }

    private val downloadClient by lazy {
        OkHttpClient.Builder()
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(10, TimeUnit.MINUTES)
            .build()
    }

    fun cancel() {
        scope.cancel()
    }

    /** Returns a playable WebViewAssetLoader URL for videoId if fully cached with a matching hash, else "". */
    fun playableUrlOrEmpty(videoId: String, expectedHash: String): String {
        val hashFile = File(cacheDir, "$videoId.hash")
        if (!hashFile.exists() || hashFile.readText().trim() != expectedHash) {
            return "" // Not cached or hash mismatch
        }
        val mp4 = File(cacheDir, "$videoId.mp4")
        val mkv = File(cacheDir, "$videoId.mkv")
        val videoFile = when {
            mp4.exists() -> mp4
            mkv.exists() -> mkv
            else -> return ""
        }
        return "https://appassets.androidplatform.net/screensaver/${videoFile.name}"
    }

    fun download(videoId: String, expectedHash: String) {
        // Fast path: already cached with matching hash
        val hashFile = File(cacheDir, "$videoId.hash")
        if (hashFile.exists() && hashFile.readText().trim() == expectedHash) {
            Log.d(TAG, "Screensaver video $videoId already cached (hash match)")
            return
        }

        scope.launch {
            try {
                val tempFile = File(cacheDir, "$videoId.tmp")
                val url = "$BASE_URL/api/v1/screensaver/videos/$videoId"

                // Resume: if temp file exists, request remaining bytes
                val existingBytes = if (tempFile.exists()) tempFile.length() else 0L
                val requestBuilder = Request.Builder().url(url)
                if (existingBytes > 0) {
                    requestBuilder.header("Range", "bytes=$existingBytes-")
                    Log.d(TAG, "Resuming screensaver download $videoId from byte $existingBytes")
                } else {
                    Log.d(TAG, "Starting screensaver download: $videoId")
                }

                val response = downloadClient.newCall(requestBuilder.build()).execute()
                if (!response.isSuccessful && response.code != 206) {
                    Log.e(TAG, "Screensaver download failed: HTTP ${response.code} for $videoId")
                    response.close()
                    return@launch
                }

                val contentType = response.header("Content-Type", "video/mp4") ?: "video/mp4"
                val ext = if (contentType.contains("matroska") || contentType.contains("mkv")) ".mkv" else ".mp4"
                val body = response.body
                if (body == null) {
                    Log.e(TAG, "Empty response body for screensaver video $videoId")
                    response.close()
                    return@launch
                }

                // Append on 206 (resume), create on 200 (fresh)
                val append = response.code == 206
                FileOutputStream(tempFile, append).use { output ->
                    body.byteStream().use { input ->
                        val buffer = ByteArray(131072) // 128KB
                        var bytesRead: Int
                        var downloadedBytes = 0L
                        while (input.read(buffer).also { bytesRead = it } != -1) {
                            output.write(buffer, 0, bytesRead)
                            downloadedBytes += bytesRead
                        }
                        Log.d(TAG, "Downloaded $downloadedBytes new bytes for screensaver $videoId (total=${tempFile.length()})")
                    }
                }
                response.close()

                // Verify SHA256 of the complete file
                val digest = MessageDigest.getInstance("SHA-256")
                FileInputStream(tempFile).use { fis ->
                    val buffer = ByteArray(131072)
                    var bytesRead: Int
                    while (fis.read(buffer).also { bytesRead = it } != -1) {
                        digest.update(buffer, 0, bytesRead)
                    }
                }
                val actualHash = digest.digest().joinToString("") { "%02x".format(it) }

                if (actualHash != expectedHash) {
                    Log.e(TAG, "Hash mismatch for screensaver $videoId: expected=$expectedHash, got=$actualHash — deleting")
                    tempFile.delete()
                    return@launch
                }

                // Remove any old video files for this ID
                cacheDir.listFiles()?.forEach { f ->
                    if (f.nameWithoutExtension == videoId && f.extension in listOf("mp4", "mkv")) {
                        f.delete()
                    }
                }

                // Promote temp to final
                val videoFile = File(cacheDir, "$videoId$ext")
                if (!tempFile.renameTo(videoFile)) {
                    Log.e(TAG, "Failed to rename temp file for screensaver $videoId")
                    return@launch
                }

                // Write hash marker (signals download complete)
                hashFile.writeText(expectedHash)

                Log.d(TAG, "Screensaver video ready: $videoId (${videoFile.length()} bytes, hash=$expectedHash)")
            } catch (e: Exception) {
                Log.e(TAG, "Screensaver download failed: $videoId", e)
            }
        }
    }

    fun cleanupStale(activeIds: Set<String>) {
        scope.launch {
            cacheDir.listFiles()?.forEach { f ->
                val id = f.nameWithoutExtension
                if (id !in activeIds && f.extension in listOf("mp4", "mkv", "hash", "tmp")) {
                    f.delete()
                    Log.d(TAG, "Cleaned up stale screensaver file: ${f.name}")
                }
            }
        }
    }
}
