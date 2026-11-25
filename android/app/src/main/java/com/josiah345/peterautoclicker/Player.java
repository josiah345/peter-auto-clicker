package com.josiah345.peterautoclicker;

import android.accessibilityservice.AccessibilityService;
import android.accessibilityservice.GestureDescription;
import android.content.Context;
import android.graphics.Path;
import android.os.Handler;
import android.os.Looper;
import android.os.SystemClock;

import java.util.ArrayList;
import java.util.List;
import java.util.function.IntConsumer;

/**
 * Replays recordings and tap points with the accessibility gesture API.
 * Long finger strokes are sent as chained segments so the original pace is kept:
 * Precise mode uses short 40 ms segments, Smart mode uses 120 ms segments
 * (fewer system calls, better on slower phones).
 */
final class Player {

    interface Listener {
        void onCount(int n);           // 3, 2, 1, then 0 = go
        void onDone(boolean stopped);
    }

    static final class Config {
        int repeat;
        boolean forever;
        long pauseMs;
        float speed;
        boolean smart;
        long intervalMs;
        boolean countdown;

        static Config from(Context c) {
            Config k = new Config();
            k.repeat = Math.max(1, Prefs.getInt(c, Prefs.REPEAT, 10));
            k.forever = Prefs.getBool(c, Prefs.FOREVER, false);
            k.pauseMs = Math.max(0, Prefs.getInt(c, Prefs.PAUSE, 0));
            k.speed = Math.max(0.1f, Math.min(10f, Prefs.getFloat(c, Prefs.SPEED, 1f)));
            k.smart = Prefs.getBool(c, Prefs.SMART, false);
            k.intervalMs = Math.max(0, Prefs.getInt(c, Prefs.INTERVAL, 500));
            k.countdown = Prefs.getBool(c, Prefs.COUNTDOWN, true);
            return k;
        }
    }

    private static final float TAP_SLOP_PX = 12f;
    private static final long MAX_SEG = 59_000;

    private final AccessibilityService svc;
    private final Listener listener;
    private final Handler h = new Handler(Looper.getMainLooper());
    private int gen;            // bumps on every start/stop: stale callbacks see a different value and quit
    private boolean running;

    Player(AccessibilityService svc, Listener l) {
        this.svc = svc;
        this.listener = l;
    }

    boolean running() { return running; }

    void stop() {
        if (!running) return;
        gen++;
        running = false;
        h.removeCallbacksAndMessages(null);
        listener.onDone(true);
    }

    private void finish() {
        gen++;
        running = false;
        h.removeCallbacksAndMessages(null);
        listener.onDone(false);
    }

    private void begin(Config k, IntConsumer firstLoop) {
        running = true;
        final int g = ++gen;
        if (!k.countdown) {
            firstLoop.accept(g);
            return;
        }
        for (int i = 0; i <= 3; i++) {
            final int n = 3 - i;
            h.postDelayed(() -> {
                if (g != gen) return;
                listener.onCount(n);
                if (n == 0) firstLoop.accept(g);
            }, i * 1000L);
        }
    }

    private static long scale(long ms, Config k) {
        return (long) (ms / k.speed);
    }

    private static float c(float v) {
        return Math.max(0f, v); // gesture paths may not have negative coordinates
    }

    // ───────────────────────── recordings ─────────────────────────

    void playRec(Rec r, Config k) {
        begin(k, g -> recLoop(r, k, 1, g));
    }

    private void recLoop(Rec r, Config k, int loop, int g) {
        if (g != gen) return;
        nextStroke(r, k, loop, g, SystemClock.uptimeMillis(), 0);
    }

    private void nextStroke(Rec r, Config k, int loop, int g, long base, int i) {
        if (g != gen) return;
        if (i >= r.strokes.size()) {
            h.postAtTime(() -> endRecLoop(r, k, loop, g), base + scale(r.duration, k));
            return;
        }
        Stroke st = r.strokes.get(i);
        h.postAtTime(() -> {
            if (g != gen) return;
            playStroke(st, k, g, () -> nextStroke(r, k, loop, g, base, i + 1));
        }, base + scale(st.start(), k));
    }

    private void endRecLoop(Rec r, Config k, int loop, int g) {
        if (g != gen) return;
        if (!k.forever && loop >= k.repeat) {
            finish();
            return;
        }
        h.postDelayed(() -> recLoop(r, k, loop + 1, g), k.pauseMs);
    }

    private void playStroke(Stroke st, Config k, int g, Runnable done) {
        int n = st.x.length;
        long total = st.end() - st.start();
        if (n < 2 || st.maxMove() < TAP_SLOP_PX) {
            // a tap (or a long press if the finger stayed down)
            Path p = new Path();
            p.moveTo(c(st.x[0]), c(st.y[0]));
            long d = Math.max(1, Math.min(scale(total, k), MAX_SEG));
            send(new GestureDescription.StrokeDescription(p, 0, d), g, done, done);
            return;
        }
        long chunk = k.smart ? 120 : 40;
        List<int[]> segs = new ArrayList<>();
        int a = 0;
        for (int j = 1; j < n; j++) {
            if (st.t[j] - st.t[a] >= chunk || j == n - 1) {
                segs.add(new int[]{a, j});
                a = j;
            }
        }
        chain(st, segs, 0, null, k, g, done);
    }

    private void chain(Stroke st, List<int[]> segs, int idx, GestureDescription.StrokeDescription prev,
                       Config k, int g, Runnable done) {
        if (g != gen) return;
        int[] sg = segs.get(idx);
        Path p = new Path();
        p.moveTo(c(st.x[sg[0]]), c(st.y[sg[0]]));
        for (int m = sg[0] + 1; m <= sg[1]; m++) p.lineTo(c(st.x[m]), c(st.y[m]));
        long d = Math.max(1, Math.min(scale(st.t[sg[1]] - st.t[sg[0]], k), MAX_SEG));
        boolean more = idx < segs.size() - 1;
        GestureDescription.StrokeDescription sd;
        try {
            sd = prev == null
                    ? new GestureDescription.StrokeDescription(p, 0, d, more)
                    : prev.continueStroke(p, 0, d, more);
        } catch (RuntimeException e) {
            done.run();
            return;
        }
        final GestureDescription.StrokeDescription fsd = sd;
        Runnable next = more ? () -> chain(st, segs, idx + 1, fsd, k, g, done) : done;
        send(sd, g, next, done);
    }

    // ───────────────────────── tap points ─────────────────────────

    void playSteps(List<Step> steps, Config k) {
        begin(k, g -> stepLoop(steps, k, 1, 0, g));
    }

    private void stepLoop(List<Step> steps, Config k, int loop, int i, int g) {
        if (g != gen) return;
        if (i >= steps.size()) {
            if (!k.forever && loop >= k.repeat) {
                finish();
                return;
            }
            h.postDelayed(() -> stepLoop(steps, k, loop + 1, 0, g), k.pauseMs);
            return;
        }
        Step st = steps.get(i);
        Path p = new Path();
        p.moveTo(c(st.x1), c(st.y1));
        long d = 40;
        if (st.swipe) {
            p.lineTo(c(st.x2), c(st.y2));
            d = Math.max(1, scale(350, k));
        }
        Runnable next = () -> h.postDelayed(() -> stepLoop(steps, k, loop, i + 1, g), scale(k.intervalMs, k));
        send(new GestureDescription.StrokeDescription(p, 0, d), g, next, next);
    }

    // ───────────────────────── sending ─────────────────────────

    private void send(GestureDescription.StrokeDescription sd, int g, Runnable onOk, Runnable onFail) {
        boolean ok;
        try {
            GestureDescription gd = new GestureDescription.Builder().addStroke(sd).build();
            ok = svc.dispatchGesture(gd, new AccessibilityService.GestureResultCallback() {
                @Override
                public void onCompleted(GestureDescription d) {
                    if (g == gen) onOk.run();
                }

                @Override
                public void onCancelled(GestureDescription d) {
                    if (g == gen) onFail.run();
                }
            }, h);
        } catch (RuntimeException e) {
            ok = false;
        }
        if (!ok) h.post(() -> {
            if (g == gen) onFail.run();
        });
    }
}
