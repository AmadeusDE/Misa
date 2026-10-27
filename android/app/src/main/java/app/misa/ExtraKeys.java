package app.misa;

import android.content.Context;
import android.graphics.Typeface;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.KeyEvent;
import android.view.View;
import android.widget.Button;
import android.widget.HorizontalScrollView;
import android.widget.LinearLayout;

// Esc, Ctrl, Alt, Tab, arrows and a few shell characters, above the IME.
// Ctrl and Alt latch until the next key. The IME key shows the keyboard;
// a long press switches raw and text input modes.
final class ExtraKeys extends HorizontalScrollView {
    interface Listener {
        void key(int code, int meta);

        void toggleIme();

        void toggleImeMode();

        void showSessions();
    }

    private final Listener listener;
    private final Button ctrl;
    private final Button alt;
    private final Button ime;
    private final Button sessions;
    private boolean ctrlOn;
    private boolean altOn;

    ExtraKeys(Context context, Listener listener) {
        super(context);
        this.listener = listener;
        setHorizontalScrollBarEnabled(false);
        setFillViewport(true);
        LinearLayout row = new LinearLayout(context);
        row.setOrientation(LinearLayout.HORIZONTAL);
        row.setGravity(Gravity.CENTER_VERTICAL);
        addView(row, new LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.MATCH_PARENT));

        row.addView(key("ESC", () -> send(KeyEvent.KEYCODE_ESCAPE, 0)));
        row.addView(key("TAB", () -> send(KeyEvent.KEYCODE_TAB, 0)));
        ctrl = key("CTRL", this::toggleCtrl);
        alt = key("ALT", this::toggleAlt);
        row.addView(ctrl);
        row.addView(alt);
        row.addView(key("←", () -> send(KeyEvent.KEYCODE_DPAD_LEFT, 0)));
        row.addView(key("↑", () -> send(KeyEvent.KEYCODE_DPAD_UP, 0)));
        row.addView(key("↓", () -> send(KeyEvent.KEYCODE_DPAD_DOWN, 0)));
        row.addView(key("→", () -> send(KeyEvent.KEYCODE_DPAD_RIGHT, 0)));
        row.addView(key("-", () -> send(KeyEvent.KEYCODE_MINUS, 0)));
        row.addView(key("/", () -> send(KeyEvent.KEYCODE_SLASH, 0)));
        row.addView(key("|", () -> send(KeyEvent.KEYCODE_BACKSLASH, KeyEvent.META_SHIFT_ON)));
        row.addView(key("~", () -> send(KeyEvent.KEYCODE_GRAVE, KeyEvent.META_SHIFT_ON)));
        ime = key("IME", listener::toggleIme);
        ime.setOnLongClickListener(v -> {
            listener.toggleImeMode();
            return true;
        });
        row.addView(ime);
        sessions = key("≡", listener::showSessions);
        row.addView(sessions);
        paint();
    }

    int latchedMeta() {
        int meta = 0;
        if (ctrlOn) {
            meta |= KeyEvent.META_CTRL_ON;
        }
        if (altOn) {
            meta |= KeyEvent.META_ALT_ON;
        }
        return meta;
    }

    void clearLatch() {
        if (!ctrlOn && !altOn) {
            return;
        }
        ctrlOn = false;
        altOn = false;
        paint();
    }

    void setImeRaw(boolean raw) {
        ime.setText(raw ? "IME" : "abc");
    }

    void setSessionLabel(String label) {
        sessions.setText(label);
    }

    private void toggleCtrl() {
        ctrlOn = !ctrlOn;
        paint();
    }

    private void toggleAlt() {
        altOn = !altOn;
        paint();
    }

    private void send(int code, int meta) {
        listener.key(code, meta | latchedMeta());
        clearLatch();
    }

    private void paint() {
        style(ctrl, ctrlOn);
        style(alt, altOn);
    }

    private void style(Button button, boolean on) {
        button.setBackgroundColor(getResources().getColor(on ? R.color.key_on : R.color.key_bg, getContext().getTheme()));
        button.setTextColor(getResources().getColor(on ? R.color.key_on_fg : R.color.key_fg, getContext().getTheme()));
    }

    private Button key(String label, Runnable action) {
        Button button = new Button(getContext());
        button.setText(label);
        button.setAllCaps(false);
        button.setTypeface(Typeface.MONOSPACE);
        button.setTextSize(TypedValue.COMPLEX_UNIT_SP, 13);
        button.setMinWidth(dp(36));
        button.setMinimumWidth(dp(36));
        button.setMinHeight(0);
        button.setMinimumHeight(0);
        button.setPadding(dp(8), 0, dp(8), 0);
        button.setFocusable(false);
        button.setOnClickListener(v -> action.run());
        style(button, false);
        LinearLayout.LayoutParams lp =
                new LinearLayout.LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.MATCH_PARENT);
        lp.setMargins(dp(2), dp(4), dp(2), dp(4));
        button.setLayoutParams(lp);
        return button;
    }

    private int dp(int value) {
        return Math.round(value * getResources().getDisplayMetrics().density);
    }
}
