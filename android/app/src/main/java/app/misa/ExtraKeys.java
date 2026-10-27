package app.misa;

import android.content.Context;
import android.graphics.Typeface;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.KeyEvent;
import android.widget.Button;
import android.widget.HorizontalScrollView;
import android.widget.LinearLayout;

// Esc, Ctrl, Alt, Tab, arrows and a few shell characters, above the IME.
// Ctrl and Alt latch until the next key or IME commit. MUX opens a row of
// one-tap multiplexer actions (ids are MuxChords in internal/gui/android).
// The IME key shows the keyboard; a long press switches raw and text modes.
final class ExtraKeys extends LinearLayout implements Modifiers {
    interface Listener {
        void key(int code, int meta);

        void toggleIme();

        void toggleImeMode();

        void showSessions();

        void mux(String action);
    }

    // label, muxAction id. zoom-in/zoom-out are font scale, not key chords.
    // Action ids match MuxChords / zoom-in / zoom-out in internal/gui/android.
    // internal/gui/android/mux_test.go reads this table.
    private static final String[][] MUX = {
        {"split", "split"},
        {"next", "next"},
        {"◁", "left"},
        {"▽", "down"},
        {"△", "up"},
        {"▷", "right"},
        {"close", "close"},
        {"layout", "layout"},
        {"resize", "resize"},
        {"agent", "agent"},
        {"reattach", "reattach"},
        {"copy", "copy"},
        {"paste", "paste"},
        {"^W", "prefix"},
        {"A+", "zoom-in"},
        {"A−", "zoom-out"},
        {"t1", "tag1"},
        {"t2", "tag2"},
        {"t3", "tag3"},
        {"t4", "tag4"},
        {"t5", "tag5"},
        {"t6", "tag6"},
        {"t7", "tag7"},
        {"t8", "tag8"},
        {"t9", "tag9"},
        {"t0", "tag0"},
    };

    private final Listener listener;
    private final Button ctrl;
    private final Button alt;
    private final Button ime;
    private final Button sessions;
    private final Button mux;
    private final HorizontalScrollView muxScroll;
    private boolean ctrlOn;
    private boolean altOn;
    private boolean muxOn = true;

    ExtraKeys(Context context, Listener listener) {
        super(context);
        this.listener = listener;
        setOrientation(VERTICAL);

        LinearLayout muxRow = new LinearLayout(context);
        muxRow.setOrientation(HORIZONTAL);
        muxRow.setGravity(Gravity.CENTER_VERTICAL);
        for (String[] item : MUX) {
            String action = item[1];
            muxRow.addView(key(item[0], () -> sendMux(action)));
        }
        muxScroll = new HorizontalScrollView(context);
        muxScroll.setHorizontalScrollBarEnabled(false);
        muxScroll.setFillViewport(true);
        muxScroll.addView(muxRow, new HorizontalScrollView.LayoutParams(LayoutParams.WRAP_CONTENT, dp(48)));
        addView(muxScroll, new LayoutParams(LayoutParams.MATCH_PARENT, dp(48)));

        LinearLayout row = new LinearLayout(context);
        row.setOrientation(HORIZONTAL);
        row.setGravity(Gravity.CENTER_VERTICAL);
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
        mux = key("MUX", this::toggleMux);
        row.addView(mux);

        HorizontalScrollView main = new HorizontalScrollView(context);
        main.setHorizontalScrollBarEnabled(false);
        main.setFillViewport(true);
        main.addView(row, new HorizontalScrollView.LayoutParams(LayoutParams.WRAP_CONTENT, dp(48)));
        addView(main, new LayoutParams(LayoutParams.MATCH_PARENT, dp(48)));
        paint();
    }

    @Override
    public int latchedMeta() {
        int meta = 0;
        if (ctrlOn) {
            meta |= KeyEvent.META_CTRL_ON;
        }
        if (altOn) {
            meta |= KeyEvent.META_ALT_ON;
        }
        return meta;
    }

    @Override
    public void clearLatch() {
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

    private void toggleMux() {
        muxOn = !muxOn;
        muxScroll.setVisibility(muxOn ? VISIBLE : GONE);
        paint();
    }

    private void send(int code, int meta) {
        listener.key(code, meta | latchedMeta());
        clearLatch();
    }

    private void sendMux(String action) {
        clearLatch();
        listener.mux(action);
    }

    private void paint() {
        style(ctrl, ctrlOn);
        style(alt, altOn);
        style(mux, muxOn);
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
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.MATCH_PARENT);
        lp.setMargins(dp(2), dp(4), dp(2), dp(4));
        button.setLayoutParams(lp);
        return button;
    }

    private int dp(int value) {
        return Math.round(value * getResources().getDisplayMetrics().density);
    }
}
