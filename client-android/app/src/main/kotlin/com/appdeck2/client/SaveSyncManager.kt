package com.appdeck2.client

import android.content.Context
import android.util.Log
import java.io.File
import java.util.zip.ZipEntry
import java.util.zip.ZipInputStream
import java.util.zip.ZipOutputStream
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.asRequestBody
import org.json.JSONObject

/**
 * Stages a user's WHOLE Eden (Switch emulator) save profile aside on session
 * end -- every title they have progress in at once, since Eden keeps all of
 * them under one shared on-device profile folder -- so the live path is
 * immediately clear for whoever logs in next, uploads the staged bundle to
 * the backend in the background, and always re-downloads fresh from the
 * server at login rather than trusting whatever's left on this device.
 *
 * The live save path lives inside Eden's own app-private storage
 * (Android/data/<eden package>/...), which requires root to touch. Once a
 * bundle is staged into AppDeck's own external-files storage, everything
 * else (zip, upload, download) is plain unprivileged file/network I/O.
 */
object SaveSyncManager {
    private const val TAG = "SaveSyncManager"
    private const val BASE_URL = "http://xiongmaovr.cn"
    private val EDEN_SAVE_ROOT =
        "/storage/emulated/0/Android/data/${GameLauncher.EDEN_PACKAGE}/files/nand/user/save/0000000000000000"

    private val httpClient = OkHttpClient.Builder()
        .connectTimeout(10, java.util.concurrent.TimeUnit.SECONDS)
        .readTimeout(30, java.util.concurrent.TimeUnit.SECONDS)
        .build()

    // Tracks users whose save profile has already been placed on THIS device
    // this login, so the pre-launch safety net (ensureRestoredForLaunch)
    // doesn't redundantly re-download what login-time restore already did.
    // Membership is per-userId, so a device-wide user switch just adds a new
    // entry rather than needing an explicit reset.
    private val restoredUserIds = java.util.Collections.synchronizedSet(mutableSetOf<String>())

    private fun stagingRoot(context: Context) = File(context.getExternalFilesDir(null), "save_staging")
    private fun stagingUserDir(context: Context, userId: String) = File(stagingRoot(context), userId)
    private fun stagingZipFile(context: Context, userId: String) = File(stagingRoot(context), "$userId.zip")

    private fun execSu(cmd: String): Int {
        return try {
            val proc = Runtime.getRuntime().exec(arrayOf("su", "-c", cmd))
            val stderr = proc.errorStream.bufferedReader().readText()
            val exit = proc.waitFor()
            if (exit != 0) Log.w(TAG, "shell failed ($exit): $cmd -- $stderr")
            exit
        } catch (e: Exception) {
            Log.e(TAG, "shell exec failed: $cmd", e)
            -1
        }
    }

    /**
     * Forces the staging root to exist and be world-writable via root,
     * regardless of who created it or what state it's currently in --
     * plain mkdirs() is a no-op if the directory already exists, so it
     * can't fix a root left root-owned by an earlier attempt (e.g. a prior
     * su-driven mkdir under it). Cheap enough to call on every entry point
     * that touches staging.
     */
    private fun ensureStagingRootWritable(context: Context) {
        val root = stagingRoot(context)
        execSu("mkdir -p '${root.absolutePath}' && chmod 777 '${root.absolutePath}'")
    }

    /** Asks the backend whether an admin has save sync turned on right now. */
    private fun isSyncEnabled(hostId: String, machineCode: String): Boolean {
        return try {
            val request = Request.Builder()
                .url("$BASE_URL/api/v1/client/config/save_sync_enabled?host_id=$hostId&machine_code=$machineCode")
                .get()
                .build()
            httpClient.newCall(request).execute().use { response ->
                if (!response.isSuccessful) return true // fail open: don't strand local saves over a network hiccup
                JSONObject(response.body?.string() ?: "{}")
                    .optJSONObject("data")?.optBoolean("enabled", true) ?: true
            }
        } catch (e: Exception) {
            Log.w(TAG, "isSyncEnabled check failed, assuming enabled", e)
            true
        }
    }

    /**
     * Finds this device's Eden save-data folder -- the UUID-named directory
     * directly under the shared profile root (e.g. CA0681678A...), which
     * differs per device/install. Returns null only if Eden has never
     * created any save index at all on this device.
     */
    private fun findSaveDataFolder(): String? {
        return try {
            val proc = Runtime.getRuntime().exec(
                arrayOf("su", "-c", "find $EDEN_SAVE_ROOT -mindepth 1 -maxdepth 1 -type d")
            )
            val out = proc.inputStream.bufferedReader().readText().trim()
            proc.waitFor()
            out.lineSequence().firstOrNull { it.isNotBlank() }
        } catch (e: Exception) {
            Log.e(TAG, "findSaveDataFolder failed", e)
            null
        }
    }

    private fun hasLiveSaveData(saveDataFolder: String): Boolean {
        return try {
            val proc = Runtime.getRuntime().exec(
                arrayOf("su", "-c", "find '$saveDataFolder' -mindepth 1 -maxdepth 1")
            )
            val out = proc.inputStream.bufferedReader().readText().trim()
            proc.waitFor()
            out.isNotEmpty()
        } catch (e: Exception) {
            Log.e(TAG, "hasLiveSaveData failed for $saveDataFolder", e)
            false
        }
    }

    /**
     * Called when the backend reports a session ended during which at least
     * one Eden title was played. Moves the user's ENTIRE save-data folder
     * (every title at once) out of Eden's live path immediately -- so the
     * next login starts clean -- zips it, and kicks off a background upload.
     */
    fun stageAndUploadUser(context: Context, userId: String, hostId: String, machineCode: String) {
        if (!isSyncEnabled(hostId, machineCode)) {
            Log.d(TAG, "stageAndUploadUser: save sync disabled, leaving live save as-is")
            return
        }

        val saveDataFolder = findSaveDataFolder()
        if (saveDataFolder == null) {
            Log.d(TAG, "stageAndUploadUser: no save index on this device, nothing to stage")
            return
        }
        if (!hasLiveSaveData(saveDataFolder)) {
            Log.d(TAG, "stageAndUploadUser: save-data folder empty, nothing to stage")
            return
        }

        val userStageDir = stagingUserDir(context, userId)
        ensureStagingRootWritable(context)
        // Eden's save files come in owned by Eden's UID with rw-rw---- perms,
        // which our own unprivileged zipDirectory() call below can't read
        // without the chmod.
        val moveExit = execSu(
            "rm -rf '${userStageDir.absolutePath}' && mkdir -p '${userStageDir.absolutePath}' && " +
                "mv '$saveDataFolder'/* '${userStageDir.absolutePath}/' && " +
                "chmod -R 777 '${userStageDir.absolutePath}'"
        )
        if (moveExit != 0) {
            Log.e(TAG, "stageAndUploadUser: move-out failed for user=$userId, leaving live save untouched")
            return
        }
        // Live save-data folder is now empty -- next login starts clean.

        try {
            zipDirectory(userStageDir, stagingZipFile(context, userId))
        } catch (e: Exception) {
            Log.e(TAG, "stageAndUploadUser: zip failed for user=$userId", e)
            return
        } finally {
            userStageDir.deleteRecursively() // redundant with the zip now
        }

        Log.d(TAG, "Staged save bundle for user=$userId, uploading in background")
        Thread { uploadUserZip(context, userId, hostId, machineCode) }.start()
    }

    /**
     * Uploads the staged zip for [userId], if any. Deletes it once the
     * upload is confirmed -- a failed upload leaves the zip in place for the
     * next cold-start retry, so a save is never lost, only delayed.
     * Returns true if there's nothing left to do (uploaded now, already
     * gone, or nothing was staged).
     */
    fun uploadUserZip(context: Context, userId: String, hostId: String, machineCode: String): Boolean {
        val zipFile = stagingZipFile(context, userId)
        if (!zipFile.exists()) return true

        return try {
            val body = MultipartBody.Builder().setType(MultipartBody.FORM)
                .addFormDataPart("file", "$userId.zip", zipFile.asRequestBody("application/zip".toMediaType()))
                .build()

            val request = Request.Builder()
                .url("$BASE_URL/api/v1/client/saves/$userId?host_id=$hostId&machine_code=$machineCode")
                .post(body)
                .build()

            httpClient.newCall(request).execute().use { response ->
                if (response.isSuccessful) {
                    zipFile.delete()
                    Log.d(TAG, "Uploaded save bundle for user=$userId")
                    true
                } else {
                    Log.w(TAG, "Upload failed for user=$userId: HTTP ${response.code}")
                    false
                }
            }
        } catch (e: Exception) {
            Log.e(TAG, "uploadUserZip failed for user=$userId", e)
            false
        }
    }

    /**
     * Runs at EVERY app cold start (not gated to once/day): retries the
     * upload for every leftover staged zip found on disk. A save that still
     * fails to upload is left in place for the next cold start.
     */
    fun retryIncompleteUploads(context: Context, hostId: String, machineCode: String) {
        Thread {
            try {
                if (!isSyncEnabled(hostId, machineCode)) {
                    Log.d(TAG, "retryIncompleteUploads: save sync disabled, skipping for now")
                    return@Thread
                }
                val root = stagingRoot(context)
                val zipFiles = root.listFiles { f -> f.isFile && f.name.endsWith(".zip") } ?: emptyArray()
                for (zipFile in zipFiles) {
                    val userId = zipFile.name.removeSuffix(".zip")
                    val uploaded = uploadUserZip(context, userId, hostId, machineCode)
                    if (uploaded) {
                        Log.d(TAG, "retryIncompleteUploads: cleared leftover save for user=$userId")
                    } else {
                        Log.w(TAG, "retryIncompleteUploads: user=$userId still not uploaded, keeping for next start")
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "retryIncompleteUploads failed", e)
            }
        }.start()
    }

    /**
     * Wipes whatever's currently live and always re-downloads this user's
     * save bundle fresh from the server -- no "prefer local staging" fast
     * path, since local staging is deleted as soon as an upload confirms and
     * shouldn't be trusted otherwise. Places the result ("patches" it) at
     * Eden's save-data folder. Runs synchronously on the calling thread;
     * callers that don't want to block should wrap this in their own
     * background thread/coroutine.
     *
     * [onProgress] fires once up front (completed=0,total=1) and once at the
     * end (done=true) so the UI can show a "restoring save" tip.
     */
    private fun doRestore(
        context: Context,
        userId: String,
        hostId: String,
        machineCode: String,
        onProgress: (completed: Int, total: Int, done: Boolean) -> Unit
    ) {
        try {
            if (!isSyncEnabled(hostId, machineCode)) {
                Log.d(TAG, "doRestore: save sync disabled, skipping for user=$userId")
                return
            }

            val saveDataFolder = findSaveDataFolder()
            if (saveDataFolder == null) {
                Log.e(TAG, "doRestore: no save index on this device yet (Eden never launched here) -- " +
                    "cannot place a restore, skipping for user=$userId")
                return
            }

            onProgress(0, 1, false)

            // Always wipe live first -- never merge with, or assume freshness
            // of, whatever happens to be sitting there already.
            execSu("rm -rf '$saveDataFolder'/*")

            val request = Request.Builder()
                .url("$BASE_URL/api/v1/client/saves/$userId?host_id=$hostId&machine_code=$machineCode")
                .get()
                .build()
            httpClient.newCall(request).execute().use { response ->
                if (response.code == 404) {
                    Log.d(TAG, "doRestore: no save on server yet for user=$userId")
                    restoredUserIds.add(userId)
                    return@use
                }
                if (!response.isSuccessful) {
                    Log.w(TAG, "doRestore: download failed for user=$userId: HTTP ${response.code}")
                    return@use // leave restoredUserIds unmarked so a retry can happen
                }
                val bytes = response.body?.bytes()
                if (bytes == null) {
                    Log.w(TAG, "doRestore: empty response body for user=$userId")
                    return@use
                }

                val tempDir = stagingUserDir(context, userId)
                ensureStagingRootWritable(context)
                tempDir.deleteRecursively()
                tempDir.mkdirs()
                unzipInto(bytes, tempDir)

                // cp -a run as root preserves our own app's ownership on the
                // copied files rather than Eden's -- chmod so Eden (whatever
                // UID it actually runs as) can still read/write its own save
                // data afterward.
                val placeExit = execSu("cp -a '${tempDir.absolutePath}/.' '$saveDataFolder/' && chmod -R 777 '$saveDataFolder'")
                tempDir.deleteRecursively()

                if (placeExit == 0) {
                    Log.d(TAG, "Restored save bundle for user=$userId")
                    restoredUserIds.add(userId)
                } else {
                    Log.e(TAG, "doRestore: placing into live path failed for user=$userId")
                }
            }
        } catch (e: Exception) {
            Log.e(TAG, "doRestore failed for user=$userId", e)
        } finally {
            onProgress(1, 1, true)
        }
    }

    /** Called at login. Runs the full wipe+download+patch in the background. */
    fun restoreForLogin(
        context: Context,
        userId: String,
        hostId: String,
        machineCode: String,
        onProgress: (completed: Int, total: Int, done: Boolean) -> Unit = { _, _, _ -> }
    ) {
        Thread { doRestore(context, userId, hostId, machineCode, onProgress) }.start()
    }

    /**
     * Safety net called right before launching Eden. If login-time restore
     * for this user already completed on this device, this is a no-op;
     * otherwise (e.g. Eden launched before the async login restore landed)
     * it runs the same wipe+download+patch synchronously so Eden never
     * starts against a stale or half-restored save.
     */
    fun ensureRestoredForLaunch(context: Context, userId: String, hostId: String, machineCode: String) {
        if (restoredUserIds.contains(userId)) return
        doRestore(context, userId, hostId, machineCode) { _, _, _ -> }
    }

    private fun unzipInto(zipBytes: ByteArray, destDir: File) {
        val destCanonical = destDir.canonicalPath
        ZipInputStream(zipBytes.inputStream()).use { zis ->
            var entry = zis.nextEntry
            while (entry != null) {
                if (!entry.isDirectory) {
                    val outFile = File(destDir, entry.name)
                    if (!outFile.canonicalPath.startsWith(destCanonical + File.separator)) {
                        Log.w(TAG, "unzipInto: rejecting entry outside destination: ${entry.name}")
                    } else {
                        outFile.parentFile?.mkdirs()
                        outFile.outputStream().use { zis.copyTo(it) }
                    }
                }
                zis.closeEntry()
                entry = zis.nextEntry
            }
        }
    }

    private fun zipDirectory(sourceDir: File, zipFile: File) {
        ZipOutputStream(zipFile.outputStream()).use { zos ->
            sourceDir.walkTopDown().filter { it.isFile }.forEach { file ->
                val entryName = file.relativeTo(sourceDir).path
                zos.putNextEntry(ZipEntry(entryName))
                file.inputStream().use { it.copyTo(zos) }
                zos.closeEntry()
            }
        }
    }
}
