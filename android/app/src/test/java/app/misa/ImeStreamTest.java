package app.misa;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;

public class ImeStreamTest {
    private static final int CTRL = 0x1000;
    private static final int ALT = 0x2;
    private static final int KEY_W = 51;

    @Test
    public void commitTextAppliesLatchOnce() {
        ImeStream stream = new ImeStream();
        ImeStream.Batch first = stream.commitText("w", CTRL);
        assertTrue(first.consumed);
        assertEquals(1, first.events.size());
        assertText(first.events.get(0), "w", CTRL);

        ImeStream.Batch second = stream.commitText("w", 0);
        assertFalse(second.consumed);
        assertEquals(1, second.events.size());
        assertText(second.events.get(0), "w", 0);

        ImeStream.Batch word = stream.commitText("bc", ALT);
        assertTrue(word.consumed);
        assertText(word.events.get(0), "bc", ALT);
    }

    @Test
    public void composingCommitIsNotSentTwice() {
        ImeStream stream = new ImeStream();
        ImeStream.Batch composing = stream.setComposingText("w", CTRL);
        assertTrue(composing.consumed);
        assertText(composing.events.get(0), "w", CTRL);

        stream.finishComposingText();
        ImeStream.Batch commit = stream.commitText("w", 0);
        assertEquals(0, commit.events.size());

        ImeStream.Batch again = stream.commitText("w", 0);
        assertText(again.events.get(0), "w", 0);
    }

    @Test
    public void composingGrowthSendsOnlyTheSuffix() {
        ImeStream stream = new ImeStream();
        stream.setComposingText("w", CTRL);
        ImeStream.Batch grown = stream.setComposingText("wo", 0);
        assertEquals(1, grown.events.size());
        assertText(grown.events.get(0), "o", 0);

        ImeStream.Batch commit = stream.commitText("wo", 0);
        assertEquals(0, commit.events.size());
    }

    @Test
    public void plainComposingIsRewritten() {
        ImeStream stream = new ImeStream();
        ImeStream.Batch first = stream.setComposingText("h", 0);
        assertText(first.events.get(0), "h", 0);

        ImeStream.Batch second = stream.setComposingText("he", 0);
        assertEquals(2, second.events.size());
        assertKey(second.events.get(0), ImeStream.KEYCODE_DEL, 0, true);
        assertText(second.events.get(1), "he", 0);

        ImeStream.Batch commit = stream.commitText("hey", 0);
        assertEquals(3, commit.events.size());
        assertKey(commit.events.get(0), ImeStream.KEYCODE_DEL, 0, true);
        assertKey(commit.events.get(1), ImeStream.KEYCODE_DEL, 0, true);
        assertText(commit.events.get(2), "hey", 0);
    }

    @Test
    public void keyAndDeleteUseTheLatch() {
        ImeStream stream = new ImeStream();
        ImeStream.Batch down = stream.key(KEY_W, 0, true, false, CTRL);
        assertTrue(down.consumed);
        assertKey(down.events.get(0), KEY_W, CTRL, true);
        ImeStream.Batch up = stream.key(KEY_W, 0, false, false, CTRL);
        assertFalse(up.consumed);
        assertKey(up.events.get(0), KEY_W, 0, false);

        ImeStream held = new ImeStream();
        ImeStream.Batch mod = held.key(113, 0, true, true, CTRL);
        assertFalse(mod.consumed);
        assertKey(mod.events.get(0), 113, 0, true);

        ImeStream.Batch del = stream.deleteSurrounding(2, ALT);
        assertTrue(del.consumed);
        assertEquals(2, del.events.size());
        assertKey(del.events.get(0), ImeStream.KEYCODE_DEL, ALT, true);
        assertKey(del.events.get(1), ImeStream.KEYCODE_DEL, 0, true);
    }

    @Test
    public void emptyCommitDoesNotConsumeTheLatch() {
        ImeStream stream = new ImeStream();
        ImeStream.Batch batch = stream.commitText("", CTRL);
        assertFalse(batch.consumed);
        assertEquals(0, batch.events.size());
    }

    private static void assertText(ImeStream.Event event, String text, int meta) {
        assertTrue(event.isText());
        assertEquals(text, event.text);
        assertEquals(meta, event.meta);
    }

    private static void assertKey(ImeStream.Event event, int code, int meta, boolean down) {
        assertFalse(event.isText());
        assertEquals(code, event.code);
        assertEquals(meta, event.meta);
        assertEquals(down, event.down);
    }
}
