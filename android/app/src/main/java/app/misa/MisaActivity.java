package app.misa;

import android.Manifest;
import android.annotation.SuppressLint;
import android.app.Activity;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.pm.PackageManager;
import android.graphics.Color;
import android.graphics.Insets;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.os.Handler;
import android.os.Looper;
import android.provider.Settings;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.KeyEvent;
import android.view.View;
import android.view.WindowInsets;
import android.view.WindowManager;
import android.view.inputmethod.InputMethodManager;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.HorizontalScrollView;
import android.widget.LinearLayout;

import java.util.ArrayList;

public final class MisaActivity extends Activity implements ExtraKeys.Listener {
    private static final int REQ_NOTIFICATIONS = 1;
    private static final int REQ_STORAGE = 2;

    private final Handler handler = new Handler(Looper.getMainLooper());
    private final ArrayList<Integer> sessionIds = new ArrayList<>();
    private final BroadcastReceiver exitReceiver = new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
            finish();
        }
    };

    private TerminalView terminal;
    private ExtraKeys keys;
    private LinearLayout bottom;
    private LinearLayout sessionRow;
    private boolean sessionsOpen;
    private int insetLeft;
    private int insetTop;
    private int insetRight;
    private int insetBottom;
    private boolean imeVisible;

    private final Runnable tick = new Runnable() {
        @Override
        public void run() {
            if (keys != null) {
                keys.setSessionLabel(sessionLabel());
            }
            if (sessionsOpen) {
                int count = Native.sessionCount();
                if (count != sessionIds.size()) {
                    reconcileSessions();
                    fillSessionRow();
                }
            }
            handler.postDelayed(this, 1000);
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        edgeToEdge();
        Bootstrap.ensureNative(this);
        if (Native.sessionCount() == 0) {
            openSession();
        } else {
            reconcileSessions();
        }
        buildUi();
        registerExit();
        requestNotifications();
        startSessionService();
        handler.post(tick);
    }

    @Override
    protected void onDestroy() {
        handler.removeCallbacks(tick);
        unregisterReceiver(exitReceiver);
        super.onDestroy();
    }

    @Override
    public void onTrimMemory(int level) {
        super.onTrimMemory(level);
        if (level >= TRIM_MEMORY_RUNNING_LOW) {
            Native.onLowMemory();
        }
    }

    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        int code = event.getKeyCode();
        if (code == KeyEvent.KEYCODE_VOLUME_UP
                || code == KeyEvent.KEYCODE_VOLUME_DOWN
                || code == KeyEvent.KEYCODE_VOLUME_MUTE) {
            return super.dispatchKeyEvent(event);
        }
        if (code == KeyEvent.KEYCODE_BACK && Build.VERSION.SDK_INT < 33) {
            if (event.getAction() == KeyEvent.ACTION_UP) {
                onBackRequest();
            }
            return true;
        }
        boolean down = event.getAction() == KeyEvent.ACTION_DOWN;
        int meta = event.getMetaState() | keys.latchedMeta();
        Native.onKey(code, meta, down);
        if (down) {
            keys.clearLatch();
        }
        return true;
    }

    @Override
    public void key(int code, int meta) {
        Native.onKey(code, meta, true);
        Native.onKey(code, meta, false);
    }

    @Override
    public void toggleIme() {
        terminal.toggleIme();
    }

    @Override
    public void toggleImeMode() {
        terminal.setRawIme(!terminal.rawIme());
        keys.setImeRaw(terminal.rawIme());
    }

    @Override
    public void showSessions() {
        sessionsOpen = !sessionsOpen;
        sessionRow.setVisibility(sessionsOpen ? View.VISIBLE : View.GONE);
        if (sessionsOpen) {
            reconcileSessions();
            fillSessionRow();
        }
        bottom.requestLayout();
    }

    private void edgeToEdge() {
        WindowManager.LayoutParams attrs = getWindow().getAttributes();
        if (Build.VERSION.SDK_INT >= 28) {
            attrs.layoutInDisplayCutoutMode =
                    WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
            getWindow().setAttributes(attrs);
        }
        if (Build.VERSION.SDK_INT >= 30) {
            getWindow().setDecorFitsSystemWindows(false);
        } else {
            @SuppressWarnings("deprecation")
            int flags = View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                    | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                    | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION;
            getWindow().getDecorView().setSystemUiVisibility(flags);
        }
        if (Build.VERSION.SDK_INT >= 33) {
            getOnBackInvokedDispatcher()
                    .registerOnBackInvokedCallback(
                            android.window.OnBackInvokedDispatcher.PRIORITY_DEFAULT, this::onBackRequest);
        }
        getWindow().setStatusBarColor(Color.TRANSPARENT);
        getWindow().setNavigationBarColor(Color.BLACK);
    }

    private void buildUi() {
        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(Color.BLACK);
        terminal = new TerminalView(this);
        terminal.setGeometry((w, h) -> pushGeometry());
        root.addView(
                terminal, new FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT));

        bottom = new LinearLayout(this);
        bottom.setOrientation(LinearLayout.VERTICAL);
        bottom.setBackgroundColor(getColor(R.color.bar));
        sessionRow = new LinearLayout(this);
        sessionRow.setOrientation(LinearLayout.HORIZONTAL);
        sessionRow.setVisibility(View.GONE);
        HorizontalScrollView sessionScroll = new HorizontalScrollView(this);
        sessionScroll.setHorizontalScrollBarEnabled(false);
        sessionScroll.addView(sessionRow);
        bottom.addView(
                sessionScroll,
                new LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT));
        keys = new ExtraKeys(this, this);
        keys.setSessionLabel(sessionLabel());
        bottom.addView(
                keys,
                new LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.MATCH_PARENT, dp(48)));
        FrameLayout.LayoutParams barLp = new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.WRAP_CONTENT, Gravity.BOTTOM);
        root.addView(bottom, barLp);
        bottom.addOnLayoutChangeListener(
                (v, left, top, right, bottomPx, oldLeft, oldTop, oldRight, oldBottom) -> pushGeometry());
        root.setOnApplyWindowInsetsListener(this::onInsets);
        setContentView(root);
        terminal.requestFocus();
    }

    private WindowInsets onInsets(View view, WindowInsets insets) {
        if (Build.VERSION.SDK_INT >= 30) {
            imeVisible = insets.isVisible(WindowInsets.Type.ime());
            Insets bars = insets.getInsets(WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout());
            Insets ime = insets.getInsets(WindowInsets.Type.ime());
            insetLeft = Math.max(bars.left, ime.left);
            insetTop = Math.max(bars.top, ime.top);
            insetRight = Math.max(bars.right, ime.right);
            insetBottom = Math.max(bars.bottom, ime.bottom);
        } else {
            imeVisible = false;
            insetLeft = insets.getSystemWindowInsetLeft();
            insetTop = insets.getSystemWindowInsetTop();
            insetRight = insets.getSystemWindowInsetRight();
            insetBottom = insets.getSystemWindowInsetBottom();
        }
        FrameLayout.LayoutParams lp = (FrameLayout.LayoutParams) bottom.getLayoutParams();
        if (lp.bottomMargin != insetBottom || lp.leftMargin != insetLeft || lp.rightMargin != insetRight) {
            lp.bottomMargin = insetBottom;
            lp.leftMargin = insetLeft;
            lp.rightMargin = insetRight;
            bottom.setLayoutParams(lp);
        }
        pushGeometry();
        return insets;
    }

    private void pushGeometry() {
        if (terminal == null || terminal.width() <= 0 || terminal.height() <= 0) {
            return;
        }
        int overlay = bottom.getHeight();
        float density = getResources().getDisplayMetrics().density;
        Native.onSurfaceChanged(
                terminal.width(),
                terminal.height(),
                density,
                insetLeft,
                insetTop,
                insetRight,
                insetBottom + overlay);
    }

    private void onBackRequest() {
        InputMethodManager imm = getSystemService(InputMethodManager.class);
        if (imeVisible) {
            if (imm != null) {
                imm.hideSoftInputFromWindow(terminal.getWindowToken(), 0);
            }
            return;
        }
        if (imm != null && imm.hideSoftInputFromWindow(terminal.getWindowToken(), 0)) {
            return;
        }
        moveTaskToBack(true);
    }

    private void openSession() {
        int id = Native.newSession();
        if (id != 0) {
            sessionIds.add(id);
        }
    }

    private void startSessionService() {
        Intent intent = new Intent(this, SessionService.class);
        startForegroundService(intent);
    }

    private void reconcileSessions() {
        int count = Native.sessionCount();
        int current = Native.activeSession();
        if (sessionIds.size() == count && (count == 0 || sessionIds.contains(current))) {
            return;
        }
        ArrayList<Integer> live = new ArrayList<>();
        for (int id = 1; live.size() < count && id < 100000; id++) {
            if (id == current) {
                live.add(id);
                continue;
            }
            Native.switchSession(id);
            if (Native.activeSession() == id) {
                live.add(id);
            }
        }
        if (current != 0 && Native.activeSession() != current) {
            Native.switchSession(current);
        }
        sessionIds.clear();
        sessionIds.addAll(live);
    }

    private void fillSessionRow() {
        sessionRow.removeAllViews();
        sessionRow.addView(textButton(getString(R.string.new_session), v -> {
            openSession();
            startSessionService();
            fillSessionRow();
        }));
        sessionRow.addView(textButton(getString(R.string.close_session), v -> {
            int id = Native.activeSession();
            if (id != 0) {
                Native.closeSession(id);
                sessionIds.remove(Integer.valueOf(id));
            }
            fillSessionRow();
        }));
        int active = Native.activeSession();
        for (int i = 0; i < sessionIds.size(); i++) {
            int id = sessionIds.get(i);
            Button button = textButton(Integer.toString(id), v -> {
                Native.switchSession(id);
                fillSessionRow();
            });
            if (id == active) {
                button.setBackgroundColor(getColor(R.color.key_on));
                button.setTextColor(getColor(R.color.key_on_fg));
            }
            sessionRow.addView(button);
        }
        String storage = Bootstrap.storageLinked(this) ? getString(R.string.storage_linked) : getString(R.string.link_storage);
        sessionRow.addView(textButton(storage, v -> linkStorage()));
        keys.setSessionLabel(sessionLabel());
    }

    private String sessionLabel() {
        int active = Native.activeSession();
        int count = Native.sessionCount();
        if (count <= 0) {
            return "≡";
        }
        return active + "/" + count;
    }

    private Button textButton(String label, View.OnClickListener click) {
        Button button = new Button(this);
        button.setText(label);
        button.setAllCaps(false);
        button.setTextSize(TypedValue.COMPLEX_UNIT_SP, 13);
        button.setMinWidth(dp(36));
        button.setMinimumWidth(dp(36));
        button.setMinHeight(0);
        button.setMinimumHeight(0);
        button.setPadding(dp(8), dp(4), dp(8), dp(4));
        button.setFocusable(false);
        button.setBackgroundColor(getColor(R.color.key_bg));
        button.setTextColor(getColor(R.color.key_fg));
        button.setOnClickListener(click);
        LinearLayout.LayoutParams lp =
                new LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, LinearLayout.LayoutParams.WRAP_CONTENT);
        lp.setMargins(dp(2), dp(4), dp(2), dp(4));
        button.setLayoutParams(lp);
        return button;
    }

    // RECEIVER_NOT_EXPORTED exists on API 33. The older overload is only used below that.
    @SuppressLint("UnspecifiedRegisterReceiverFlag")
    private void registerExit() {
        IntentFilter filter = new IntentFilter(SessionService.ACTION_EXIT);
        if (Build.VERSION.SDK_INT >= 33) {
            registerReceiver(exitReceiver, filter, RECEIVER_NOT_EXPORTED);
        } else {
            registerReceiver(exitReceiver, filter);
        }
    }

    private void requestNotifications() {
        if (Build.VERSION.SDK_INT < 33) {
            return;
        }
        if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[] {Manifest.permission.POST_NOTIFICATIONS}, REQ_NOTIFICATIONS);
        }
    }

    private void linkStorage() {
        if (Build.VERSION.SDK_INT >= 30) {
            if (Environment.isExternalStorageManager()) {
                Bootstrap.linkSharedStorage(this);
                fillSessionRow();
                return;
            }
            try {
                Intent intent = new Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION);
                intent.setData(Uri.parse("package:" + getPackageName()));
                startActivityForResult(intent, REQ_STORAGE);
            } catch (android.content.ActivityNotFoundException e) {
                startActivityForResult(new Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION), REQ_STORAGE);
            }
            return;
        }
        if (checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED) {
            Bootstrap.linkSharedStorage(this);
            fillSessionRow();
            return;
        }
        requestPermissions(
                new String[] {Manifest.permission.READ_EXTERNAL_STORAGE, Manifest.permission.WRITE_EXTERNAL_STORAGE},
                REQ_STORAGE);
    }

    @Override
    @SuppressWarnings("deprecation")
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == REQ_STORAGE && Build.VERSION.SDK_INT >= 30 && Environment.isExternalStorageManager()) {
            Bootstrap.linkSharedStorage(this);
            if (sessionsOpen) {
                fillSessionRow();
            }
        }
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode != REQ_STORAGE) {
            return;
        }
        for (int result : grantResults) {
            if (result == PackageManager.PERMISSION_GRANTED) {
                Bootstrap.linkSharedStorage(this);
                if (sessionsOpen) {
                    fillSessionRow();
                }
                return;
            }
        }
    }

    private int dp(int value) {
        return Math.round(value * getResources().getDisplayMetrics().density);
    }
}
