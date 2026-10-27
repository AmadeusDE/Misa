package app.misa;

import android.content.Context;
import android.view.MotionEvent;
import android.view.ScaleGestureDetector;
import android.view.SurfaceHolder;
import android.view.SurfaceView;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputMethodManager;

// Surface the Go renderer draws, plus the soft keyboard and pinch-to-zoom.
public final class TerminalView extends SurfaceView
        implements SurfaceHolder.Callback, ScaleGestureDetector.OnScaleGestureListener {
    interface Geometry {
        void onSurface(int width, int height);
    }

    private final ScaleGestureDetector scale;
    private Geometry geometry;
    private int width;
    private int height;
    private boolean rawIme = true;

    public TerminalView(Context context) {
        super(context);
        getHolder().addCallback(this);
        setFocusable(true);
        setFocusableInTouchMode(true);
        setImportantForAutofill(IMPORTANT_FOR_AUTOFILL_NO);
        scale = new ScaleGestureDetector(context, this);
    }

    void setGeometry(Geometry geometry) {
        this.geometry = geometry;
    }

    int width() {
        return width;
    }

    int height() {
        return height;
    }

    boolean rawIme() {
        return rawIme;
    }

    void setRawIme(boolean raw) {
        rawIme = raw;
        InputMethodManager imm = getContext().getSystemService(InputMethodManager.class);
        if (imm != null) {
            imm.restartInput(this);
        }
    }

    @SuppressWarnings("deprecation")
    void toggleIme() {
        InputMethodManager imm = getContext().getSystemService(InputMethodManager.class);
        if (imm == null) {
            return;
        }
        requestFocus();
        imm.toggleSoftInput(InputMethodManager.SHOW_FORCED, InputMethodManager.HIDE_IMPLICIT_ONLY);
    }

    @Override
    public boolean onCheckIsTextEditor() {
        return true;
    }

    @Override
    public InputConnection onCreateInputConnection(EditorInfo outAttrs) {
        if (rawIme) {
            outAttrs.inputType = EditorInfo.TYPE_NULL;
        } else {
            outAttrs.inputType = EditorInfo.TYPE_CLASS_TEXT
                    | EditorInfo.TYPE_TEXT_FLAG_NO_SUGGESTIONS
                    | EditorInfo.TYPE_TEXT_VARIATION_VISIBLE_PASSWORD;
        }
        outAttrs.imeOptions =
                EditorInfo.IME_FLAG_NO_FULLSCREEN | EditorInfo.IME_FLAG_NO_EXTRACT_UI | EditorInfo.IME_ACTION_NONE;
        return new MisaInputConnection(this);
    }

    @Override
    public void surfaceCreated(SurfaceHolder holder) {
        Native.onSurfaceCreated(holder.getSurface());
    }

    @Override
    public void surfaceChanged(SurfaceHolder holder, int format, int w, int h) {
        width = w;
        height = h;
        if (geometry != null) {
            geometry.onSurface(w, h);
        }
    }

    @Override
    public void surfaceDestroyed(SurfaceHolder holder) {
        width = 0;
        height = 0;
        Native.onSurfaceDestroyed();
    }

    @Override
    public boolean onTouchEvent(MotionEvent event) {
        scale.onTouchEvent(event);
        int action = event.getActionMasked();
        if (!(scale.isInProgress() && action == MotionEvent.ACTION_MOVE)) {
            Native.onMotion(action, event.getX(), event.getY(), event.getButtonState());
        }
        if (action == MotionEvent.ACTION_UP) {
            performClick();
        }
        return true;
    }

    @Override
    public boolean performClick() {
        super.performClick();
        return true;
    }

    @Override
    public boolean onGenericMotionEvent(MotionEvent event) {
        if (event.getActionMasked() == MotionEvent.ACTION_SCROLL) {
            float horizontal = event.getAxisValue(MotionEvent.AXIS_HSCROLL);
            float vertical = event.getAxisValue(MotionEvent.AXIS_VSCROLL);
            Native.onMotion(MotionEvent.ACTION_SCROLL, horizontal, -vertical, event.getButtonState());
            return true;
        }
        return super.onGenericMotionEvent(event);
    }

    @Override
    public boolean onScale(ScaleGestureDetector detector) {
        Native.onPinch(detector.getScaleFactor());
        return true;
    }

    @Override
    public boolean onScaleBegin(ScaleGestureDetector detector) {
        return true;
    }

    @Override
    public void onScaleEnd(ScaleGestureDetector detector) {}
}
