package app.misa;

import android.view.KeyEvent;
import android.view.View;
import android.view.inputmethod.BaseInputConnection;

// Forwards IME commits, composing text, deletions and key events into the
// same JNI path hardware keys use. Composing text is written to the terminal
// and rewritten when the IME replaces it.
final class MisaInputConnection extends BaseInputConnection {
    private String composing = "";

    MisaInputConnection(View target) {
        super(target, true);
    }

    @Override
    public boolean commitText(CharSequence text, int newCursorPosition) {
        deleteComposing();
        if (text != null && text.length() > 0) {
            Native.onText(text.toString());
        }
        clearEditable();
        return true;
    }

    @Override
    public boolean setComposingText(CharSequence text, int newCursorPosition) {
        deleteComposing();
        composing = text == null ? "" : text.toString();
        if (!composing.isEmpty()) {
            Native.onText(composing);
        }
        return true;
    }

    @Override
    public boolean finishComposingText() {
        composing = "";
        clearEditable();
        return super.finishComposingText();
    }

    @Override
    public boolean deleteSurroundingText(int beforeLength, int afterLength) {
        deleteKeys(beforeLength);
        return true;
    }

    @Override
    public boolean deleteSurroundingTextInCodePoints(int beforeLength, int afterLength) {
        deleteKeys(beforeLength);
        return true;
    }

    @Override
    public boolean sendKeyEvent(KeyEvent event) {
        Native.onKey(event.getKeyCode(), event.getMetaState(), event.getAction() == KeyEvent.ACTION_DOWN);
        return true;
    }

    private void deleteComposing() {
        deleteKeys(composing.codePointCount(0, composing.length()));
        composing = "";
    }

    private void deleteKeys(int count) {
        for (int i = 0; i < count; i++) {
            Native.onKey(KeyEvent.KEYCODE_DEL, 0, true);
        }
    }

    private void clearEditable() {
        if (getEditable() != null) {
            getEditable().clear();
        }
    }
}
