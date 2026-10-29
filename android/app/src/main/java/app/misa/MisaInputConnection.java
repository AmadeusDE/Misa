package app.misa;

import android.view.KeyEvent;
import android.view.View;
import android.view.inputmethod.BaseInputConnection;

// Forwards IME commits, composing text, deletions and key events into the
// same JNI path hardware keys use. A latched Ctrl or Alt from the extra-keys
// bar applies to the next commit, composing text or key, then clears.
final class MisaInputConnection extends BaseInputConnection {
    private final Modifiers modifiers;
    private final ImeStream stream = new ImeStream();

    MisaInputConnection(View target, Modifiers modifiers) {
        super(target, true);
        this.modifiers = modifiers;
    }

    @Override
    public boolean commitText(CharSequence text, int newCursorPosition) {
        apply(stream.commitText(text == null ? "" : text.toString(), latchedMeta()));
        clearEditable();
        return true;
    }

    @Override
    public boolean setComposingText(CharSequence text, int newCursorPosition) {
        apply(stream.setComposingText(text == null ? "" : text.toString(), latchedMeta()));
        return true;
    }

    @Override
    public boolean finishComposingText() {
        stream.finishComposingText();
        clearEditable();
        return super.finishComposingText();
    }

    @Override
    public boolean deleteSurroundingText(int beforeLength, int afterLength) {
        apply(stream.deleteSurrounding(beforeLength, latchedMeta()));
        return true;
    }

    @Override
    public boolean deleteSurroundingTextInCodePoints(int beforeLength, int afterLength) {
        apply(stream.deleteSurrounding(beforeLength, latchedMeta()));
        return true;
    }

    @Override
    public boolean sendKeyEvent(KeyEvent event) {
        boolean down = event.getAction() == KeyEvent.ACTION_DOWN;
        apply(stream.key(event.getKeyCode(), event.getMetaState(), down, modifierKey(event.getKeyCode()), latchedMeta()));
        return true;
    }

    private void apply(ImeStream.Batch batch) {
        for (int i = 0; i < batch.events.size(); i++) {
            ImeStream.Event event = batch.events.get(i);
            if (event.isText()) {
                Native.onText(event.text, event.meta);
            } else {
                Native.onKey(event.code, event.meta, event.down);
            }
        }
        if (batch.consumed && modifiers != null) {
            modifiers.clearLatch();
        }
    }

    private int latchedMeta() {
        return modifiers == null ? 0 : modifiers.latchedMeta();
    }

    private static boolean modifierKey(int code) {
        switch (code) {
            case KeyEvent.KEYCODE_SHIFT_LEFT:
            case KeyEvent.KEYCODE_SHIFT_RIGHT:
            case KeyEvent.KEYCODE_ALT_LEFT:
            case KeyEvent.KEYCODE_ALT_RIGHT:
            case KeyEvent.KEYCODE_CTRL_LEFT:
            case KeyEvent.KEYCODE_CTRL_RIGHT:
            case KeyEvent.KEYCODE_META_LEFT:
            case KeyEvent.KEYCODE_META_RIGHT:
                return true;
            default:
                return false;
        }
    }

    private void clearEditable() {
        if (getEditable() != null) {
            getEditable().clear();
        }
    }
}

// Modifiers is the extra-keys Ctrl/Alt latch.
interface Modifiers {
    int latchedMeta();

    void clearLatch();
}
