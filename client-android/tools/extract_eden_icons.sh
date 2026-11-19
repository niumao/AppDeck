#!/system/bin/sh
# Extract icons from Eden (Switch) XCI/NSP game files using hactool.
# Run on-device with root: su -c /data/local/tmp/extract_eden_icons.sh

GAMES_DIR="/storage/emulated/0/Documents/swi"
ICONS_DIR="/storage/emulated/0/Documents/swiicons"
KEYS_SRC="/storage/emulated/0/Android/data/com.miHoYo.Yuanshen/files/keys/prod.keys"
KEYS="/data/local/tmp/prod.keys"
HACTOOL="/data/local/tmp/hactool"
TMPDIR="/data/local/tmp/eden_extract"

# Sanity checks
if [ ! -x "$HACTOOL" ]; then
    echo "ERROR: hactool not found at $HACTOOL"
    exit 1
fi
if [ ! -f "$KEYS_SRC" ]; then
    echo "ERROR: prod.keys not found at $KEYS_SRC"
    exit 1
fi
if [ ! -d "$GAMES_DIR" ]; then
    echo "ERROR: games dir not found at $GAMES_DIR"
    exit 1
fi

# Copy keys to local storage (FUSE paths unreliable for hactool)
cp "$KEYS_SRC" "$KEYS"

mkdir -p "$ICONS_DIR"

# Helper: extract icon from NCAs in a directory
extract_icon_from_ncas() {
    local ncas_dir="$1"
    local icon_out="$2"
    local file_basename="$3"

    # Find the Control NCA by checking content type
    # Skip .cnmt.nca files (metadata), only check .nca files
    control_nca=""
    for nca in "$ncas_dir"/*.nca; do
        [ -f "$nca" ] || continue
        # Skip cnmt.nca files
        case "$nca" in *.cnmt.nca) continue;; esac

        type=$("$HACTOOL" -k "$KEYS" "$nca" 2>/dev/null | grep "Content Type")
        case "$type" in
            *Control*)
                control_nca="$nca"
                break
                ;;
        esac
    done

    if [ -z "$control_nca" ]; then
        echo "  WARN: no Control NCA found for $file_basename"
        return 1
    fi

    echo "  Control NCA: $(basename "$control_nca")"

    # Extract RomFS from Control NCA
    mkdir -p "$TMPDIR/romfs"
    "$HACTOOL" -k "$KEYS" --romfsdir="$TMPDIR/romfs" "$control_nca" >/dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo "  WARN: hactool romfs extract failed for $file_basename"
        return 1
    fi

    # Find and copy icon file
    icon_src=""
    for candidate in \
        "$TMPDIR/romfs/icon_AmericanEnglish.dat" \
        "$TMPDIR/romfs/icon_Japanese.dat" \
        "$TMPDIR/romfs/icon_SimplifiedChinese.dat" \
        "$TMPDIR/romfs/icon_TraditionalChinese.dat" \
        "$TMPDIR/romfs/icon_Korean.dat"; do
        if [ -f "$candidate" ]; then
            icon_src="$candidate"
            break
        fi
    done

    # Fallback: grab any icon_*.dat
    if [ -z "$icon_src" ]; then
        icon_src=$(ls "$TMPDIR/romfs"/icon_*.dat 2>/dev/null | head -1)
    fi

    if [ -z "$icon_src" ]; then
        echo "  WARN: no icon found in control NCA for $file_basename"
        return 1
    fi

    cp "$icon_src" "$icon_out"
    echo "  OK: $icon_out"
    return 0
}

# Process each XCI/NSP file (search recursively in subdirectories)
find "$GAMES_DIR" -type f \( -name '*.xci' -o -name '*.XCI' -o -name '*.nsp' -o -name '*.NSP' \) | while read gamefile; do
    file_basename=$(basename "$gamefile")
    name="${file_basename%.*}"
    icon_out="$ICONS_DIR/${name}.jpg"

    # Skip if icon already extracted
    if [ -f "$icon_out" ]; then
        echo "SKIP: $file_basename (icon exists)"
        continue
    fi

    echo "EXTRACT: $file_basename"

    # Clean temp dirs
    rm -rf "$TMPDIR"
    mkdir -p "$TMPDIR/secure"

    # Determine file type and extract NCAs accordingly
    case "$gamefile" in
        *.xci|*.XCI)
            # XCI: Extract secure partition NCAs
            "$HACTOOL" -t xci -k "$KEYS" --securedir="$TMPDIR/secure" "$gamefile" >/dev/null 2>&1
            if [ $? -ne 0 ]; then
                echo "  WARN: hactool xci extract failed for $file_basename"
                rm -rf "$TMPDIR"
                continue
            fi
            ;;
        *.nsp|*.NSP)
            # NSP (PFS0): Extract NCAs
            "$HACTOOL" -t pfs0 -k "$KEYS" --pfs0dir="$TMPDIR/secure" "$gamefile" >/dev/null 2>&1
            if [ $? -ne 0 ]; then
                echo "  WARN: hactool nsp extract failed for $file_basename"
                rm -rf "$TMPDIR"
                continue
            fi
            ;;
    esac

    extract_icon_from_ncas "$TMPDIR/secure" "$icon_out" "$file_basename"

    # Clean up temp for this game
    rm -rf "$TMPDIR"
done

# Final cleanup
rm -rf "$TMPDIR"
echo "DONE"
