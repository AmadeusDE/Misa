package app.misa;

import android.content.Context;
import android.util.Log;

import java.io.BufferedReader;
import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;

// Prepares the child environment (D10), filesDir/bin symlinks to libmisa.so
// (D5; nativeLibraryDir changes on update, so this runs every start), and a
// first-run config under $XDG_CONFIG_HOME/misa.
public final class Bootstrap {
    private static final String TAG = "misa";
    private static final Object LOCK = new Object();
    private static boolean nativeReady;

    private static final String DEFAULT_CONFIG =
            "# Misa configuration — see config.example.yaml for full docs.\n"
                    + "menu:\n"
                    + "  - title: Apps\n"
                    + "    desc: Terminal Applications\n"
                    + "    items:\n"
                    + "      - title: Lazygit\n"
                    + "        desc: Launch lazygit in the current directory\n"
                    + "        cmd: lazygit\n"
                    + "\n"
                    + "ascii:\n"
                    + "  # distro: auto\n"
                    + "  anime: random\n"
                    + "  # theme: default\n"
                    + "  # max_height: 16\n";

    private Bootstrap() {}

    public static void prepare(Context context) {
        synchronized (LOCK) {
            prepareLocked(context);
        }
    }

    public static void ensureNative(Context context) {
        synchronized (LOCK) {
            prepareLocked(context);
            if (!nativeReady) {
                Native.onCreate(environment(context), exe(context).getAbsolutePath());
                nativeReady = true;
            }
        }
    }

    public static File home(Context context) {
        return new File(context.getFilesDir(), "home");
    }

    public static File bin(Context context) {
        return new File(context.getFilesDir(), "bin");
    }

    public static File exe(Context context) {
        return new File(context.getApplicationInfo().nativeLibraryDir, "libmisa.so");
    }

    public static String[] environment(Context context) {
        File home = home(context);
        File cache = context.getCacheDir();
        File bin = bin(context);
        ArrayList<String> env = new ArrayList<>();
        env.add("HOME=" + home.getAbsolutePath());
        env.add("TMPDIR=" + cache.getAbsolutePath());
        env.add("XDG_CONFIG_HOME=" + new File(home, ".config").getAbsolutePath());
        env.add("XDG_DATA_HOME=" + new File(home, ".local/share").getAbsolutePath());
        env.add("XDG_STATE_HOME=" + new File(home, ".local/state").getAbsolutePath());
        env.add("XDG_CACHE_HOME=" + cache.getAbsolutePath());
        env.add("TERM=xterm-256color");
        env.add("COLORTERM=truecolor");
        env.add("LANG=en_US.UTF-8");
        env.add("PATH=" + bin.getAbsolutePath() + ":/system/bin");
        env.add("SHELL=" + new File(bin, "misa").getAbsolutePath());
        env.add("MISA_ANDROID_APP=1");
        for (String key : new String[] {"ANDROID_DATA", "ANDROID_ROOT", "ANDROID_STORAGE"}) {
            String value = System.getenv(key);
            if (value != null) {
                env.add(key + "=" + value);
            }
        }
        return env.toArray(new String[0]);
    }

    // ~/storage -> /sdcard. The caller asks for permission first.
    public static boolean linkSharedStorage(Context context) {
        synchronized (LOCK) {
            prepareLocked(context);
            Path link = home(context).toPath().resolve("storage");
            Path target = new File("/sdcard").toPath();
            try {
                replaceLink(link, target);
                return true;
            } catch (IOException e) {
                Log.e(TAG, "link shared storage", e);
                return false;
            }
        }
    }

    public static boolean storageLinked(Context context) {
        Path link = home(context).toPath().resolve("storage");
        if (!Files.isSymbolicLink(link)) {
            return false;
        }
        try {
            return Files.readSymbolicLink(link).equals(new File("/sdcard").toPath());
        } catch (IOException e) {
            return false;
        }
    }

    private static void prepareLocked(Context context) {
        File home = home(context);
        File bin = bin(context);
        try {
            Files.createDirectories(home.toPath().resolve(".config/misa"));
            Files.createDirectories(home.toPath().resolve(".local/share"));
            Files.createDirectories(home.toPath().resolve(".local/state"));
            Files.createDirectories(context.getCacheDir().toPath());
            Files.createDirectories(bin.toPath());
        } catch (IOException e) {
            Log.e(TAG, "create home", e);
            return;
        }
        Path config = home.toPath().resolve(".config/misa/config.yaml");
        if (!Files.exists(config)) {
            try {
                Files.write(config, DEFAULT_CONFIG.getBytes(StandardCharsets.UTF_8));
            } catch (IOException e) {
                Log.e(TAG, "write config", e);
            }
        }
        File exe = exe(context);
        Path target = exe.toPath();
        for (String name : multicallNames(context, exe)) {
            try {
                replaceLink(bin.toPath().resolve(name), target);
            } catch (IOException e) {
                Log.e(TAG, "symlink " + name, e);
            }
        }
    }

    private static void replaceLink(Path link, Path target) throws IOException {
        if (Files.isSymbolicLink(link)) {
            if (target.equals(Files.readSymbolicLink(link))) {
                return;
            }
            Files.delete(link);
        } else if (Files.exists(link)) {
            Files.delete(link);
        }
        Files.createSymbolicLink(link, target);
    }

    private static List<String> multicallNames(Context context, File exe) {
        LinkedHashSet<String> names = new LinkedHashSet<>();
        names.add("misa");
        names.add("ugh");
        if (!readAsset(context, names)) {
            readListCommand(exe, names);
        }
        return new ArrayList<>(names);
    }

    private static boolean readAsset(Context context, LinkedHashSet<String> names) {
        int before = names.size();
        try (InputStream in = context.getAssets().open("multicall.txt");
                BufferedReader reader =
                        new BufferedReader(new InputStreamReader(in, StandardCharsets.UTF_8))) {
            String line;
            while ((line = reader.readLine()) != null) {
                addName(names, line.trim());
            }
        } catch (IOException e) {
            return false;
        }
        return names.size() > before;
    }

    private static void readListCommand(File exe, LinkedHashSet<String> names) {
        if (!exe.isFile()) {
            return;
        }
        Process process = null;
        try {
            process = new ProcessBuilder(exe.getAbsolutePath(), "--list")
                    .redirectErrorStream(true)
                    .start();
            try (BufferedReader reader = new BufferedReader(
                    new InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8))) {
                String line;
                while ((line = reader.readLine()) != null) {
                    addName(names, line.trim());
                }
            }
            process.waitFor();
        } catch (Exception e) {
            Log.e(TAG, "libmisa --list", e);
        } finally {
            if (process != null) {
                process.destroy();
            }
        }
    }

    private static void addName(LinkedHashSet<String> names, String name) {
        if (name.isEmpty() || name.equals(".") || name.equals("..")) {
            return;
        }
        for (int i = 0; i < name.length(); i++) {
            char c = name.charAt(i);
            boolean ok = (c >= 'a' && c <= 'z')
                    || (c >= 'A' && c <= 'Z')
                    || (c >= '0' && c <= '9')
                    || c == '.'
                    || c == '_'
                    || c == '+'
                    || c == '-';
            if (!ok) {
                return;
            }
        }
        names.add(name);
    }
}
