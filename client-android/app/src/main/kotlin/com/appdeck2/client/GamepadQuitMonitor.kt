package com.appdeck2.client

import android.util.Log

/**
 * Monitors raw gamepad input for the L1+R1+START quit combo.
 *
 * Deploys a detached root shell script that uses `getevent` to read parsed
 * input events from the gamepad device. The script runs as an independent
 * root daemon (via setsid), immune to Oppo HANS process freezing.
 *
 * When the combo is detected, the script force-stops the foreground app
 * (detected via dumpsys, no file IPC needed). Since AppDeck is the HOME
 * launcher, Android automatically returns to it after the game is killed —
 * no explicit `am start` required.
 */
class GamepadQuitMonitor(private val onQuitCombo: () -> Unit) {

    companion object {
        private const val TAG = "GamepadQuitMonitor"
        private const val SCRIPT_PATH = "/data/local/tmp/gamepad_quit_monitor.sh"
        private const val PID_PATH = "/data/local/tmp/gamepad_quit_monitor.pid"
        const val CURRENT_GAME_PATH = "/data/local/tmp/current_game_pkg"

        // Our package — never kill ourselves
        private const val OUR_PACKAGE = "com.appdeck2.client"

        // The shell script that runs as a detached root daemon.
        // Uses getevent (line-buffered parsed output) instead of cat|xxd|awk pipeline.
        // Detects L1(0x136)+R1(0x137)+START(0x13b) combo and kills the foreground game.
        // No file IPC needed — detects foreground app via dumpsys.
        // No `am start` needed — AppDeck is HOME launcher, Android returns to it automatically.
        private val MONITOR_SCRIPT = """
#!/system/bin/sh
echo ${'$'}${'$'} > $PID_PATH
log -t GamepadQuitMonitor "Daemon started (pid=${'$'}${'$'})"

# Find all gamepad devices from /proc/bus/input/devices
find_gamepads() {
    local devs=""
    local name=""
    local handler=""
    while IFS= read -r line; do
        case "${'$'}line" in
            N:\ Name=*)
                name="${'$'}{line#N: Name=}"
                ;;
            H:\ Handlers=*)
                handler="${'$'}{line#H: Handlers=}"
                ;;
            "")
                case "${'$'}name" in
                    *X-Box*|*Xbox*|*Gamepad*|*gamepad*|*Controller*|*controller*|*Joystick*|*joystick*|*Game\ Pad*|*game\ pad*)
                        for h in ${'$'}handler; do
                            case "${'$'}h" in event*)
                                devs="${'$'}devs /dev/input/${'$'}h"
                                break
                                ;;
                            esac
                        done
                        ;;
                esac
                name=""
                handler=""
                ;;
        esac
    done < /proc/bus/input/devices
    echo ${'$'}devs
}

# Get the foreground app package from dumpsys
get_foreground_pkg() {
    dumpsys activity activities 2>/dev/null | grep -m1 'topResumedActivity' | sed 's/.*u0 \([^ /]*\).*/\1/'
}

# Kill foreground game. Uses current_game_pkg file if available (faster),
# falls back to dumpsys detection.
kill_foreground_game() {
    local pkg=""
    if [ -f $CURRENT_GAME_PATH ]; then
        pkg=${'$'}(cat $CURRENT_GAME_PATH 2>/dev/null)
        rm -f $CURRENT_GAME_PATH
    fi
    if [ -z "${'$'}pkg" ]; then
        pkg=${'$'}(get_foreground_pkg)
    fi
    if [ -n "${'$'}pkg" ] && [ "${'$'}pkg" != "$OUR_PACKAGE" ]; then
        am force-stop "${'$'}pkg"
        log -t GamepadQuitMonitor "Killed ${'$'}pkg"
    else
        log -t GamepadQuitMonitor "No game to kill (fg=${'$'}pkg)"
    fi
}

# Monitors ONE gamepad device: its own getevent process and its own
# l1/r1/start/select state. Devices are never combined into a single
# getevent call — this device's getevent binary only accepts a single
# positional [device] argument (confirmed on-device: passing two paths
# prints the usage text and exits immediately, killing combo detection
# outright whenever 2+ controllers are connected). Running one instance
# per device also stops button state from bleeding between controllers,
# since identical controller models report identical BTN_* codes with
# nothing in the event stream to tell them apart except which getevent
# process (i.e. which device) produced them.
monitor_device() {
    DEV="${'$'}1"
    TAG="${'$'}{DEV##*/}"
    COMBO_FLAG="/data/local/tmp/gqm_combo_hit_${'$'}TAG"
    SELECT_FLAG="/data/local/tmp/gqm_select_held_${'$'}TAG"

    while true; do
        rm -f "${'$'}COMBO_FLAG" "${'$'}SELECT_FLAG"

        # Watchdog: getevent has been observed to wedge into a CPU-spinning state
        # (state R, wchan 0, ~100% CPU) that never delivers another event to its
        # downstream reader, silently breaking combo-quit and volume detection
        # until the app is restarted. Sample its CPU ticks twice a second apart;
        # if it burned >80 of the last 100 ticks (i.e. >80% CPU) it's wedged, not
        # idle-blocked (a healthy idle getevent uses ~0% CPU). Kill it so the
        # existing "device stream ended, restarting" path below reconnects fresh.
        (
            while true; do
                sleep 5
                GEPID=${'$'}(pgrep -f "^getevent -ql ${'$'}DEV${'$'}")
                [ -z "${'$'}GEPID" ] && break
                read -r _ _ _ _ _ _ _ _ _ _ _ _ _ UT1 ST1 _ < /proc/${'$'}GEPID/stat 2>/dev/null || break
                sleep 1
                read -r _ _ _ _ _ _ _ _ _ _ _ _ _ UT2 ST2 _ < /proc/${'$'}GEPID/stat 2>/dev/null || break
                if [ ${'$'}(( (UT2 + ST2) - (UT1 + ST1) )) -gt 80 ]; then
                    log -t GamepadQuitMonitor "getevent wedged on ${'$'}DEV (spinning), forcing restart"
                    kill -9 "${'$'}GEPID" 2>/dev/null
                    break
                fi
            done
        ) &
        WATCHDOG_PID=${'$'}!

        log -t GamepadQuitMonitor "Monitoring ${'$'}DEV"
        l1=0
        r1=0
        start=0
        select_down=0

        # getevent -ql: quiet (no header), labeled codes and values.
        # Output format (3 columns, whitespace-separated):
        #   EV_KEY       BTN_TL               DOWN
        #   EV_KEY       BTN_TR               UP
        #   EV_ABS       ABS_RX               00001234
        # For EV_KEY, value is labeled as DOWN/UP (not hex).
        # Pipe into while-read subshell. Variables persist within the subshell
        # across loop iterations. A flag file signals the outer loop for cooldown.
        getevent -ql "${'$'}DEV" | while read -r type code value; do
            # SELECT + D-pad Up/Down: step volume while SELECT is held.
            # Cancels the plain 3s-hold-shows-panel timer since the step itself shows the panel.
            if [ "${'$'}type" = "EV_ABS" ] && [ "${'$'}code" = "ABS_HAT0Y" ] && [ "${'$'}select_down" = "1" ]; then
                case "${'$'}value" in
                    ffffffff)
                        rm -f "${'$'}SELECT_FLAG"
                        input keyevent 24
                        log -t GamepadQuitMonitor "SELECT+UP — volume up (${'$'}DEV)"
                        ;;
                    00000001)
                        rm -f "${'$'}SELECT_FLAG"
                        input keyevent 25
                        log -t GamepadQuitMonitor "SELECT+DOWN — volume down (${'$'}DEV)"
                        ;;
                esac
                continue
            fi

            [ "${'$'}type" = "EV_KEY" ] || continue

            case "${'$'}value" in
                DOWN) v=1 ;;
                UP)   v=0 ;;
                *)    continue ;;
            esac

            case "${'$'}code" in
                BTN_TL)      l1=${'$'}v ;;
                BTN_TR)      r1=${'$'}v ;;
                BTN_START)   start=${'$'}v ;;
                BTN_SELECT)
                    if [ "${'$'}v" = "1" ]; then
                        select_down=1
                        touch "${'$'}SELECT_FLAG"
                        (sleep 3; if [ -f "${'$'}SELECT_FLAG" ]; then
                            rm -f "${'$'}SELECT_FLAG"
                            input keyevent 24
                            log -t GamepadQuitMonitor "SELECT held 3s — volume panel shown (${'$'}DEV)"
                        fi) &
                    else
                        select_down=0
                        rm -f "${'$'}SELECT_FLAG"
                    fi
                    continue
                    ;;
                *)           continue ;;
            esac

            if [ "${'$'}l1" = "1" ] && [ "${'$'}r1" = "1" ] && [ "${'$'}start" = "1" ]; then
                kill_foreground_game
                log -t GamepadQuitMonitor "Combo handled on ${'$'}DEV, cooldown 5s"
                touch "${'$'}COMBO_FLAG"
                # Kill the getevent process feeding this pipe (looked up by its
                # unique device arg, not $$ -- this function runs backgrounded
                # as its own subshell, and $$ isn't reliably that subshell's own
                # pid across shells) to break out and discard buffered events.
                GEPID=${'$'}(pgrep -f "^getevent -ql ${'$'}DEV${'$'}")
                [ -n "${'$'}GEPID" ] && kill -9 "${'$'}GEPID" 2>/dev/null
                break
            fi
        done

        kill ${'$'}WATCHDOG_PID 2>/dev/null

        if [ -f "${'$'}COMBO_FLAG" ]; then
            rm -f "${'$'}COMBO_FLAG"
            sleep 5
        else
            # getevent exited (device disconnected or watchdog killed a wedged
            # process), retry after short delay
            log -t GamepadQuitMonitor "Device stream ended for ${'$'}DEV, restarting..."
            sleep 3
        fi
    done
}

# Outer loop: rescan for gamepads periodically and spawn an independent
# monitor_device worker for each newly seen device path. Workers loop
# forever internally (their own reconnect/cooldown handling), so this loop
# only needs to notice newly hot-plugged controllers, not restart existing
# ones. All workers are backgrounded children of this daemon process, so
# killExisting()'s process-group kill (-PGID) still tears down everything.
KNOWN_DEVICES=""

while true; do
    DEVICES=${'$'}(find_gamepads)
    if [ -z "${'$'}DEVICES" ]; then
        log -t GamepadQuitMonitor "No gamepad found, retrying..."
        sleep 5
        continue
    fi

    for DEV in ${'$'}DEVICES; do
        case " ${'$'}KNOWN_DEVICES " in
            *" ${'$'}DEV "*) ;;
            *)
                log -t GamepadQuitMonitor "New gamepad: ${'$'}DEV"
                monitor_device "${'$'}DEV" &
                KNOWN_DEVICES="${'$'}KNOWN_DEVICES ${'$'}DEV"
                ;;
        esac
    done

    sleep 5
done
        """.trimIndent()
    }

    fun start() {
        Thread({
            try {
                // Kill any existing monitor
                killExisting()

                // Write script to device
                Log.d(TAG, "Deploying monitor script to $SCRIPT_PATH")
                val writeProc = Runtime.getRuntime().exec(arrayOf("su", "-c",
                    "cat > $SCRIPT_PATH && chmod 755 $SCRIPT_PATH"))
                writeProc.outputStream.use { it.write(MONITOR_SCRIPT.toByteArray()) }
                writeProc.waitFor()

                // Launch as detached daemon (setsid creates new session, not a child of our process group)
                Log.d(TAG, "Launching detached daemon")
                val launchProc = Runtime.getRuntime().exec(arrayOf("su", "-c",
                    "setsid sh $SCRIPT_PATH </dev/null >/dev/null 2>/dev/null &"))
                launchProc.waitFor()

                Log.d(TAG, "Daemon launched")
            } catch (e: Exception) {
                Log.e(TAG, "Failed to start monitor daemon", e)
            }
        }, "GamepadQuitMonitor-deploy").apply {
            isDaemon = true
            start()
        }
    }

    fun stop() {
        try {
            killExisting()
        } catch (e: Exception) {
            Log.e(TAG, "Failed to stop monitor daemon", e)
        }
    }

    // The daemon process has been observed to disappear entirely under real
    // usage (root cause not pinned down — not an app-level stop(), not a
    // device reboot; likely OOM reaping of an unmanaged root process). Since
    // there's no Android lifecycle callback for "my root shell daemon died",
    // callers must poll this and call start() again if it returns false.
    fun isAlive(): Boolean {
        return try {
            val proc = Runtime.getRuntime().exec(arrayOf("su", "-c",
                "[ -f $PID_PATH ] && kill -0 \$(cat $PID_PATH) 2>/dev/null && echo alive"))
            val output = proc.inputStream.bufferedReader().readText().trim()
            proc.waitFor()
            output == "alive"
        } catch (e: Exception) {
            false
        }
    }

    fun ensureRunning() {
        if (!isAlive()) {
            Log.w(TAG, "Daemon not running, restarting")
            start()
        }
    }

    private fun killExisting() {
        try {
            // Kill entire process group (shell + getevent child)
            // setsid makes the PID the PGID, so kill -PGID kills everything
            //
            // The fallback below (for a stale/missing PID file) can't use `pkill -f` with
            // any literal pattern: this whole command runs as `su -c "<this text>"`, so
            // whatever pattern string is written here is *itself* embedded verbatim in that
            // process's own command line — pkill -f then always matches (and SIGKILLs) its
            // own invoking process, no matter how the pattern is chosen (verified on-device:
            // the command dies right at the pkill line, every time, regardless of anchoring).
            // Instead, enumerate matching PIDs with pgrep and explicitly skip our own $$.
            val proc = Runtime.getRuntime().exec(arrayOf("su", "-c",
                "if [ -f $PID_PATH ]; then kill -9 -- -\$(cat $PID_PATH) 2>/dev/null; rm -f $PID_PATH; fi; " +
                "for p in \$(pgrep -f $SCRIPT_PATH 2>/dev/null); do [ \"\$p\" = \"\$\$\" ] || kill -9 \"\$p\" 2>/dev/null; done; " +
                "rm -f /data/local/tmp/gqm_combo_hit; true"))
            proc.waitFor()
        } catch (_: Exception) {}
    }
}
