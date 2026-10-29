package app.misa;

import java.util.ArrayList;

// ImeStream decides what an InputConnection delivers to the terminal.
// A latched Ctrl/Alt applies to the next commit, composing text, deletion
// or key, then the caller clears it. A modified composing string is a key
// (not inserted text): the following commit of that same string is dropped,
// and a longer composing update sends only the new suffix.
final class ImeStream {
    static final int KEYCODE_DEL = 67;

    static final class Event {
        final String text;
        final int code;
        final int meta;
        final boolean down;

        private Event(String text, int code, int meta, boolean down) {
            this.text = text;
            this.code = code;
            this.meta = meta;
            this.down = down;
        }

        static Event text(String text, int meta) {
            return new Event(text, 0, meta, false);
        }

        static Event key(int code, int meta, boolean down) {
            return new Event(null, code, meta, down);
        }

        boolean isText() {
            return text != null;
        }
    }

    static final class Batch {
        final ArrayList<Event> events = new ArrayList<>();
        boolean consumed;
    }

    private String composing = "";
    private String armed = "";
    private int modified;

    Batch commitText(String text, int latched) {
        if (text == null) {
            text = "";
        }
        Batch batch = new Batch();
        if (!armed.isEmpty() && armed.equals(text)) {
            armed = "";
            composing = "";
            modified = 0;
            return batch;
        }
        armed = "";
        deletePlain(batch);
        composing = "";
        modified = 0;
        if (!text.isEmpty()) {
            batch.events.add(Event.text(text, latched));
            batch.consumed = latched != 0;
        }
        return batch;
    }

    Batch setComposingText(String text, int latched) {
        if (text == null) {
            text = "";
        }
        Batch batch = new Batch();
        if (latched != 0) {
            deletePlain(batch);
            if (!text.isEmpty() && !text.equals(armed)) {
                batch.events.add(Event.text(text, latched));
                batch.consumed = true;
                armed = text;
            } else if (text.isEmpty()) {
                armed = "";
            }
            composing = text;
            modified = text.codePointCount(0, text.length());
            return batch;
        }
        if (modified > 0) {
            extendModified(text, batch);
            return batch;
        }
        deletePlain(batch);
        composing = text;
        modified = 0;
        armed = "";
        if (!text.isEmpty()) {
            batch.events.add(Event.text(text, 0));
        }
        return batch;
    }

    void finishComposingText() {
        composing = "";
        modified = 0;
    }

    Batch deleteSurrounding(int count, int latched) {
        Batch batch = new Batch();
        if (count <= 0) {
            return batch;
        }
        int meta = 0;
        if (latched != 0) {
            meta = latched;
            batch.consumed = true;
            armed = "";
        }
        for (int i = 0; i < count; i++) {
            batch.events.add(Event.key(KEYCODE_DEL, i == 0 ? meta : 0, true));
        }
        return batch;
    }

    Batch key(int code, int eventMeta, boolean down, boolean modifier, int latched) {
        Batch batch = new Batch();
        int meta = eventMeta;
        if (down && !modifier && latched != 0) {
            meta |= latched;
            batch.consumed = true;
            armed = "";
        }
        batch.events.add(Event.key(code, meta, down));
        return batch;
    }

    private void extendModified(String text, Batch batch) {
        int prefixChars = prefixLength();
        String prefix = composing.substring(0, prefixChars);
        if (text.startsWith(prefix)) {
            String oldPlain = composing.substring(prefixChars);
            String added = text.substring(prefixChars);
            if (!oldPlain.equals(added)) {
                deleteText(oldPlain, batch);
                if (!added.isEmpty()) {
                    batch.events.add(Event.text(added, 0));
                }
            }
            composing = text;
            armed = text;
            return;
        }
        deleteText(composing.substring(prefixChars), batch);
        if (!text.isEmpty()) {
            batch.events.add(Event.text(text, 0));
        }
        composing = text;
        modified = 0;
        armed = text;
    }

    private int prefixLength() {
        int cps = composing.codePointCount(0, composing.length());
        int keep = modified;
        if (keep > cps) {
            keep = cps;
        }
        if (keep < 0) {
            keep = 0;
        }
        return composing.offsetByCodePoints(0, keep);
    }

    private void deletePlain(Batch batch) {
        int cps = composing.codePointCount(0, composing.length());
        int plain = cps - modified;
        if (plain < 0) {
            plain = 0;
        }
        for (int i = 0; i < plain; i++) {
            batch.events.add(Event.key(KEYCODE_DEL, 0, true));
        }
    }

    private static void deleteText(String text, Batch batch) {
        int count = text.codePointCount(0, text.length());
        for (int i = 0; i < count; i++) {
            batch.events.add(Event.key(KEYCODE_DEL, 0, true));
        }
    }
}
