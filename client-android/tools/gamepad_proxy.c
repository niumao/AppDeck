/*
 * gamepad_proxy — evdev proxy for bypassing EVIOCGRAB by games (e.g. Eden/yuzu).
 *
 * Grabs the real gamepad, creates a uinput virtual clone, forwards all events.
 * Monitors for L1+R1+START combo and outputs "QUIT\n" to stdout.
 * Also writes /data/local/tmp/gamepad-quit-signal as fallback.
 * Outputs "READY\n" once the virtual device is active and the real device is grabbed.
 *
 * Usage: gamepad_proxy [/dev/input/eventX]
 *   If no device path given, auto-detects the first gamepad with BTN_TL+BTN_TR+BTN_START.
 *
 * Build (static, for Android aarch64):
 *   aarch64-linux-gnu-gcc -static -O2 -o gamepad-proxy gamepad_proxy.c
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
#include <signal.h>
#include <dirent.h>
#include <sys/ioctl.h>
#include <linux/input.h>
#include <linux/uinput.h>

static volatile int running = 1;

static void sighandler(int sig) {
    (void)sig;
    running = 0;
}

/* Check if an evdev device supports a given key code */
static int has_key(int fd, int code) {
    unsigned char bits[(KEY_MAX + 7) / 8];
    memset(bits, 0, sizeof(bits));
    if (ioctl(fd, EVIOCGBIT(EV_KEY, sizeof(bits)), bits) < 0)
        return 0;
    return (bits[code / 8] >> (code % 8)) & 1;
}

/* Find first gamepad device that has BTN_TL + BTN_TR + BTN_START */
static int find_gamepad(char *path, size_t pathlen) {
    DIR *dir = opendir("/dev/input");
    if (!dir) return -1;

    struct dirent *ent;
    while ((ent = readdir(dir)) != NULL) {
        if (strncmp(ent->d_name, "event", 5) != 0)
            continue;

        char devpath[256];
        snprintf(devpath, sizeof(devpath), "/dev/input/%s", ent->d_name);

        int fd = open(devpath, O_RDONLY);
        if (fd < 0) continue;

        int ok = has_key(fd, BTN_TL) && has_key(fd, BTN_TR) && has_key(fd, BTN_START);
        close(fd);

        if (ok) {
            strncpy(path, devpath, pathlen - 1);
            path[pathlen - 1] = '\0';
            closedir(dir);
            return 0;
        }
    }
    closedir(dir);
    return -1;
}

int main(int argc, char *argv[]) {
    char devpath[256];

    if (argc >= 2) {
        strncpy(devpath, argv[1], sizeof(devpath) - 1);
        devpath[sizeof(devpath) - 1] = '\0';
    } else {
        if (find_gamepad(devpath, sizeof(devpath)) < 0) {
            fprintf(stderr, "ERROR: No gamepad found\n");
            return 1;
        }
    }

    signal(SIGINT, sighandler);
    signal(SIGTERM, sighandler);

    /* Open real device */
    int fd = open(devpath, O_RDWR);
    if (fd < 0) {
        fprintf(stderr, "ERROR: open %s: %s\n", devpath, strerror(errno));
        return 1;
    }

    char name[256] = "Unknown";
    ioctl(fd, EVIOCGNAME(sizeof(name)), name);

    struct input_id devid;
    memset(&devid, 0, sizeof(devid));
    ioctl(fd, EVIOCGID, &devid);

    fprintf(stderr, "Device: %s (%s)\n", name, devpath);

    /* Open uinput */
    int uifd = open("/dev/uinput", O_WRONLY | O_NONBLOCK);
    if (uifd < 0) {
        fprintf(stderr, "ERROR: open /dev/uinput: %s\n", strerror(errno));
        close(fd);
        return 1;
    }

    /* Copy supported event types and codes from real device to uinput */
    unsigned char evbits[(EV_MAX + 7) / 8];
    memset(evbits, 0, sizeof(evbits));
    ioctl(fd, EVIOCGBIT(0, sizeof(evbits)), evbits);

    int i, j;
    for (i = 0; i <= EV_MAX; i++) {
        if (!((evbits[i / 8] >> (i % 8)) & 1))
            continue;
        ioctl(uifd, UI_SET_EVBIT, i);

        int max_code = 0;
        unsigned long set_cmd = 0;

        switch (i) {
        case EV_KEY: max_code = KEY_MAX; set_cmd = UI_SET_KEYBIT; break;
        case EV_ABS: max_code = ABS_MAX; set_cmd = UI_SET_ABSBIT; break;
        case EV_REL: max_code = REL_MAX; set_cmd = UI_SET_RELBIT; break;
        case EV_MSC: max_code = MSC_MAX; set_cmd = UI_SET_MSCBIT; break;
        case EV_FF:  max_code = FF_MAX;  set_cmd = UI_SET_FFBIT;  break;
        default: continue;
        }

        int bytecount = (max_code + 7) / 8;
        unsigned char codebits[(KEY_MAX + 7) / 8]; /* KEY_MAX is largest */
        memset(codebits, 0, sizeof(codebits));
        ioctl(fd, EVIOCGBIT(i, bytecount), codebits);

        for (j = 0; j <= max_code; j++) {
            if ((codebits[j / 8] >> (j % 8)) & 1)
                ioctl(uifd, set_cmd, j);
        }
    }

    /* Configure uinput device identity */
    struct uinput_setup setup;
    memset(&setup, 0, sizeof(setup));
    snprintf(setup.name, UINPUT_MAX_NAME_SIZE, "%s (proxy)", name);
    setup.id = devid;
    setup.id.bustype = BUS_VIRTUAL;

    if (ioctl(uifd, UI_DEV_SETUP, &setup) < 0) {
        fprintf(stderr, "ERROR: UI_DEV_SETUP: %s\n", strerror(errno));
        close(uifd);
        close(fd);
        return 1;
    }

    /* Copy ABS axis parameters */
    unsigned char absbits[(ABS_MAX + 7) / 8];
    memset(absbits, 0, sizeof(absbits));
    ioctl(fd, EVIOCGBIT(EV_ABS, sizeof(absbits)), absbits);

    for (j = 0; j <= ABS_MAX; j++) {
        if (!((absbits[j / 8] >> (j % 8)) & 1))
            continue;
        struct input_absinfo ai;
        if (ioctl(fd, EVIOCGABS(j), &ai) == 0) {
            struct uinput_abs_setup as;
            memset(&as, 0, sizeof(as));
            as.code = j;
            as.absinfo = ai;
            ioctl(uifd, UI_ABS_SETUP, &as);
        }
    }

    /* Create the virtual device */
    if (ioctl(uifd, UI_DEV_CREATE) < 0) {
        fprintf(stderr, "ERROR: UI_DEV_CREATE: %s\n", strerror(errno));
        close(uifd);
        close(fd);
        return 1;
    }

    /*
     * Wait for Android InputReader to detect the virtual device
     * before we grab the real one (otherwise there's a gap with no input).
     */
    usleep(500000);

    /* Grab the real device exclusively */
    if (ioctl(fd, EVIOCGRAB, 1) < 0) {
        fprintf(stderr, "ERROR: EVIOCGRAB: %s\n", strerror(errno));
        ioctl(uifd, UI_DEV_DESTROY);
        close(uifd);
        close(fd);
        return 1;
    }

    /* Signal readiness */
    fprintf(stdout, "READY\n");
    fflush(stdout);
    fprintf(stderr, "Proxy active: %s -> virtual device\n", devpath);

    /* Button state for combo detection */
    int btn_tl = 0, btn_tr = 0, btn_start = 0;

    /* Main event loop */
    struct input_event ev;
    while (running) {
        ssize_t n = read(fd, &ev, sizeof(ev));
        if (n != (ssize_t)sizeof(ev)) {
            if (errno == EINTR) continue;
            break;
        }

        /* Forward event to virtual device */
        write(uifd, &ev, sizeof(ev));

        /* Track L1/R1/START for quit combo */
        if (ev.type == EV_KEY) {
            switch (ev.code) {
            case BTN_TL:    btn_tl    = ev.value; break;
            case BTN_TR:    btn_tr    = ev.value; break;
            case BTN_START: btn_start = ev.value; break;
            }

            if (btn_tl && btn_tr && btn_start) {
                fprintf(stdout, "QUIT\n");
                fflush(stdout);
                /* Write signal file as fallback */
                FILE *sf = fopen("/data/local/tmp/gamepad-quit-signal", "w");
                if (sf) { fprintf(sf, "QUIT\n"); fclose(sf); }
                /* Reset to avoid repeated triggers */
                btn_tl = btn_tr = btn_start = 0;
            }
        }
    }

    /* Cleanup */
    ioctl(fd, EVIOCGRAB, 0);
    ioctl(uifd, UI_DEV_DESTROY);
    close(uifd);
    close(fd);

    fprintf(stderr, "Proxy stopped\n");
    return 0;
}
