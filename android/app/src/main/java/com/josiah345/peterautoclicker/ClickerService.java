package com.josiah345.peterautoclicker;

import android.accessibilityservice.AccessibilityService;
import android.annotation.SuppressLint;
import android.content.Context;
import android.content.Intent;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.graphics.PixelFormat;
import android.graphics.Rect;
import android.os.Build;
import android.os.SystemClock;
import android.util.DisplayMetrics;
import android.view.Gravity;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.View;
import android.view.ViewGroup;
import android.view.WindowManager;
import android.view.accessibility.AccessibilityEvent;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.widget.Toast;

import java.util.ArrayList;
import java.util.Locale;

/**
 * The accessibility service: shows the floating panel, records your taps and swipes on a
 * see-through layer, places tap points, and plays everything back.
 */
public class ClickerService extends AccessibilityService implements Player.Listener {

    static ClickerService instance;

    private WindowManager wm;
    private Sounds sounds;
    private Player player;

    private LinearLayout panel;
    private WindowManager.LayoutParams panelLp;
    private ImageView bMode, bPrimary, bSwipe, bMinus, bPlay, bStop, bClose;

    private RecordView recView;
    private TextView countView;
    private LinesView linesView;
    private final ArrayList<View> markers = new ArrayList<>();
    private final ArrayList<WindowManager.LayoutParams> markerLps = new ArrayList<>();

    private boolean pointsMode, recording, panelShown;
    private long recStart;
    private Rec rec;
    private ArrayList<Step> steps;

    private static final int ROLE_NORMAL = 0, ROLE_REC = 1, ROLE_PLAY = 2, ROLE_STOP = 3;

    // ───────────────────────── lifecycle ─────────────────────────

    @Override
    protected void onServiceConnected() {
        super.onServiceConnected();
        instance = this;
        wm = (WindowManager) getSystemService(Context.WINDOW_SERVICE);
        sounds = new Sounds(this);
        player = new Player(this, this);
        rec = Rec.load(this);
        steps = Step.loadList(this);
        pointsMode = Prefs.getBool(this, Prefs.POINTS_MODE, false);
    }

    @Override
    public void onAccessibilityEvent(AccessibilityEvent event) {
    }

    @Override
    public void onInterrupt() {
    }

    @Override
    public boolean onUnbind(Intent intent) {
        cleanup();
        instance = null;
        return super.onUnbind(intent);
    }

    @Override
    public void onDestroy() {
        cleanup();
        instance = null;
        if (sounds != null) sounds.release();
        super.onDestroy();
    }

    /** Volume Down stops playback at any time. */
    @Override
    protected boolean onKeyEvent(KeyEvent e) {
        if (player != null && player.running() && e.getKeyCode() == KeyEvent.KEYCODE_VOLUME_DOWN) {
            if (e.getAction() == KeyEvent.ACTION_DOWN) player.stop();
            return true;
        }
        return false;
    }

    private void cleanup() {
        if (player != null) player.stop();
        hidePanel();
        removeCount();
    }

    // ───────────────────────── helpers ─────────────────────────

    private int dp(float v) { return Ui.dp(this, v); }

    private WindowManager.LayoutParams lp(int w, int h, boolean touchable) {
        int flags = WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE
                | WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN
                | WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS;
        if (!touchable) flags |= WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE;
        WindowManager.LayoutParams p = new WindowManager.LayoutParams(w, h,
                WindowManager.LayoutParams.TYPE_ACCESSIBILITY_OVERLAY, flags, PixelFormat.TRANSLUCENT);
        p.gravity = Gravity.TOP | Gravity.START;
        if (Build.VERSION.SDK_INT >= 28) {
            p.layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
        }
        return p;
    }

    @SuppressWarnings("deprecation")
    private int[] screen() {
        if (Build.VERSION.SDK_INT >= 30) {
            Rect b = wm.getCurrentWindowMetrics().getBounds();
            return new int[]{b.width(), b.height()};
        }
        DisplayMetrics m = new DisplayMetrics();
        wm.getDefaultDisplay().getRealMetrics(m);
        return new int[]{m.widthPixels, m.heightPixels};
    }

    private void toast(String s) {
        Toast.makeText(this, s, Toast.LENGTH_SHORT).show();
    }

    private void safeRemove(View v) {
        if (v == null) return;
        try {
            wm.removeView(v);
        } catch (RuntimeException ignored) {
        }
    }

    // ───────────────────────── floating panel ─────────────────────────

    boolean isPanelShown() { return panelShown; }

    @SuppressLint("ClickableViewAccessibility")
    void showPanel() {
        if (panelShown || wm == null) return;
        panel = new LinearLayout(this);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setGravity(Gravity.CENTER_HORIZONTAL);
        panel.setPadding(dp(6), dp(6), dp(6), dp(8));
        panel.setBackground(Ui.grad(0xF01E2C58, 0xF0131E42, dp(26), Ui.BORDER, dp(1.5f)));

        ImageView grip = new ImageView(this);
        grip.setImageResource(R.drawable.ic_grip);
        Ui.tint(grip, Ui.MUTED);
        grip.setLayoutParams(new LinearLayout.LayoutParams(dp(44), dp(26)));
        grip.setPadding(dp(10), dp(3), dp(10), dp(3));
        grip.setContentDescription("Drag to move");
        grip.setOnTouchListener(new View.OnTouchListener() {
            float rx, ry;
            int sx, sy;

            @Override
            public boolean onTouch(View v, MotionEvent e) {
                switch (e.getActionMasked()) {
                    case MotionEvent.ACTION_DOWN:
                        sx = panelLp.x;
                        sy = panelLp.y;
                        rx = e.getRawX();
                        ry = e.getRawY();
                        return true;
                    case MotionEvent.ACTION_MOVE:
                        panelLp.x = sx + (int) (e.getRawX() - rx);
                        panelLp.y = sy + (int) (e.getRawY() - ry);
                        try {
                            wm.updateViewLayout(panel, panelLp);
                        } catch (RuntimeException ignored) {
                        }
                        return true;
                    default:
                        return true;
                }
            }
        });
        panel.addView(grip);

        bMode = btn(R.drawable.ic_recmode, ROLE_NORMAL, v -> toggleMode(), "Switch between Record mode and Tap-point mode");
        bPrimary = btn(R.drawable.ic_rec, ROLE_REC, v -> primary(), "Record");
        bSwipe = btn(R.drawable.ic_swipe, ROLE_NORMAL, v -> addSwipe(), "Add a swipe");
        bMinus = btn(R.drawable.ic_minus, ROLE_NORMAL, v -> removeLast(), "Remove the last point");
        bPlay = btn(R.drawable.ic_play, ROLE_PLAY, v -> play(), "Play");
        bStop = btn(R.drawable.ic_stop, ROLE_STOP, v -> stopAll(), "Stop");
        bClose = btn(R.drawable.ic_close, ROLE_NORMAL, v -> hidePanel(), "Close the panel");

        int[] s = screen();
        panelLp = lp(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT, true);
        panelLp.x = dp(8);
        panelLp.y = s[1] / 4;
        wm.addView(panel, panelLp);
        panelShown = true;
        applyMode(false);
    }

    void hidePanel() {
        if (recording) finishRecording();
        if (player != null && player.running()) player.stop();
        hideMarkers();
        if (panelShown) safeRemove(panel);
        panel = null;
        panelShown = false;
    }

    private ImageView btn(int icon, int role, View.OnClickListener l, String desc) {
        ImageView v = new ImageView(this);
        LinearLayout.LayoutParams p = new LinearLayout.LayoutParams(dp(46), dp(46));
        p.topMargin = dp(6);
        v.setLayoutParams(p);
        v.setPadding(dp(12), dp(12), dp(12), dp(12));
        v.setImageResource(icon);
        v.setContentDescription(desc);
        style(v, role);
        v.setOnClickListener(l);
        panel.addView(v);
        return v;
    }

    private void style(ImageView v, int role) {
        switch (role) {
            case ROLE_REC:
                v.setBackground(Ui.oval(Ui.RED1, Ui.RED2, Ui.GOLD1, dp(1.5f)));
                Ui.tint(v, Color.WHITE);
                break;
            case ROLE_PLAY:
                v.setBackground(Ui.oval(Ui.GREEN1, Ui.GREEN2, Ui.GOLD1, dp(1.5f)));
                Ui.tint(v, Color.WHITE);
                break;
            case ROLE_STOP:
                v.setBackground(Ui.oval(Ui.SLATE1, Ui.SLATE2, Ui.GOLD1, dp(1.5f)));
                Ui.tint(v, Color.WHITE);
                break;
            default:
                v.setBackground(Ui.oval(Ui.CARD1, Ui.CARD2, Ui.BORDER, dp(1.5f)));
                Ui.tint(v, Ui.ICON);
        }
    }

    private void enable(View v, boolean on) {
        v.setEnabled(on);
        v.setAlpha(on ? 1f : 0.35f);
    }

    private void refreshButtons() {
        if (!panelShown) return;
        boolean playing = player.running();
        boolean idle = !playing && !recording;
        enable(bMode, idle);
        enable(bPrimary, !playing);
        enable(bSwipe, idle);
        enable(bMinus, idle && !steps.isEmpty());
        boolean hasContent = pointsMode ? !steps.isEmpty() : (rec != null && !rec.isEmpty());
        enable(bPlay, idle && hasContent);
        enable(bStop, playing || recording);
        enable(bClose, true);
    }

    private void bringPanelToFront() {
        if (!panelShown) return;
        safeRemove(panel);
        wm.addView(panel, panelLp);
    }

    // ───────────────────────── modes ─────────────────────────

    private void toggleMode() {
        if (recording || player.running()) return;
        pointsMode = !pointsMode;
        Prefs.putBool(this, Prefs.POINTS_MODE, pointsMode);
        sounds.play(sounds.toggle);
        applyMode(true);
    }

    private void applyMode(boolean announce) {
        bMode.setImageResource(pointsMode ? R.drawable.ic_points : R.drawable.ic_recmode);
        bPrimary.setImageResource(pointsMode ? R.drawable.ic_plus : R.drawable.ic_rec);
        bPrimary.setContentDescription(pointsMode ? "Add a tap point" : "Record");
        style(bPrimary, pointsMode ? ROLE_NORMAL : ROLE_REC);
        bSwipe.setVisibility(pointsMode ? View.VISIBLE : View.GONE);
        bMinus.setVisibility(pointsMode ? View.VISIBLE : View.GONE);
        if (pointsMode) rebuildMarkers();
        else hideMarkers();
        refreshButtons();
        if (announce) {
            toast(pointsMode ? "Tap-point mode: + adds a tap, the arrows add a swipe. Drag them into place"
                    : "Record mode: press the red button and do your taps and swipes");
        }
    }

    private void primary() {
        if (player.running()) return;
        if (pointsMode) addTap();
        else if (recording) finishRecording();
        else startRecording();
    }

    // ───────────────────────── record mode ─────────────────────────

    private void startRecording() {
        sounds.play(sounds.rec);
        recording = true;
        recStart = SystemClock.uptimeMillis();
        recView = new RecordView(this);
        wm.addView(recView, lp(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT, true));
        bringPanelToFront();
        bPrimary.setImageResource(R.drawable.ic_stop);
        bPrimary.setContentDescription("Finish recording");
        refreshButtons();
    }

    private void finishRecording() {
        if (!recording) return;
        recording = false;
        sounds.play(sounds.finish);
        ArrayList<Stroke> got = recView != null ? recView.done : new ArrayList<>();
        safeRemove(recView);
        recView = null;
        if (panelShown) {
            bPrimary.setImageResource(R.drawable.ic_rec);
            bPrimary.setContentDescription("Record");
        }
        if (got.isEmpty()) {
            toast(rec != null && !rec.isEmpty() ? "Nothing new recorded. Last recording kept" : "Nothing was recorded. Try again");
            refreshButtons();
            return;
        }
        Rec r = new Rec();
        for (Stroke s : got) {
            long[] t = new long[s.t.length];
            for (int i = 0; i < t.length; i++) t[i] = s.t[i] - recStart;
            r.strokes.add(new Stroke(s.x, s.y, t));
        }
        r.duration = r.strokes.get(r.strokes.size() - 1).end() + 300;
        rec = r;
        rec.save(this);
        int taps = 0;
        for (Stroke s : r.strokes) if (s.maxMove() < 12f) taps++;
        toast(String.format(Locale.US, "Recorded %.1f s: %d taps, %d swipes. Press Play",
                r.duration / 1000f, taps, r.strokes.size() - taps));
        refreshButtons();
    }

    /** See-through layer that records your finger while you act out the routine. */
    private final class RecordView extends View {
        final ArrayList<Stroke> done = new ArrayList<>();
        private ArrayList<float[]> cur;      // x, y
        private ArrayList<Long> curT;        // event time (uptime ms), kept as long for precision
        private final Paint trail = new Paint(Paint.ANTI_ALIAS_FLAG);
        private final Paint dot = new Paint(Paint.ANTI_ALIAS_FLAG);
        private final Paint frame = new Paint(Paint.ANTI_ALIAS_FLAG);
        private final Paint hint = new Paint(Paint.ANTI_ALIAS_FLAG);
        private final Paint hintBg = new Paint(Paint.ANTI_ALIAS_FLAG);

        RecordView(Context c) {
            super(c);
            trail.setColor(0xCCF6D67D);
            trail.setStyle(Paint.Style.STROKE);
            trail.setStrokeWidth(dp(3));
            trail.setStrokeCap(Paint.Cap.ROUND);
            trail.setStrokeJoin(Paint.Join.ROUND);
            dot.setColor(0xCCF6D67D);
            frame.setColor(0xFFE85050);
            frame.setStyle(Paint.Style.STROKE);
            frame.setStrokeWidth(dp(4));
            hint.setColor(Ui.TEXT);
            hint.setTextSize(dp(14));
            hint.setTextAlign(Paint.Align.CENTER);
            hint.setFakeBoldText(true);
            hintBg.setColor(0xE0131E42);
        }

        private void add(MotionEvent e) {
            float ox = e.getRawX() - e.getX(), oy = e.getRawY() - e.getY();
            for (int h = 0; h < e.getHistorySize(); h++) {
                cur.add(new float[]{e.getHistoricalX(0, h) + ox, e.getHistoricalY(0, h) + oy});
                curT.add(e.getHistoricalEventTime(h));
            }
            cur.add(new float[]{e.getRawX(), e.getRawY()});
            curT.add(e.getEventTime());
        }

        @SuppressLint("ClickableViewAccessibility")
        @Override
        public boolean onTouchEvent(MotionEvent e) {
            switch (e.getActionMasked()) {
                case MotionEvent.ACTION_DOWN:
                    cur = new ArrayList<>();
                    curT = new ArrayList<>();
                    add(e);
                    break;
                case MotionEvent.ACTION_MOVE:
                    if (cur != null) add(e);
                    break;
                case MotionEvent.ACTION_UP:
                case MotionEvent.ACTION_CANCEL:
                    if (cur != null) {
                        add(e);
                        int n = cur.size();
                        float[] x = new float[n], y = new float[n];
                        long[] t = new long[n];
                        for (int i = 0; i < n; i++) {
                            float[] p = cur.get(i);
                            x[i] = p[0];
                            y[i] = p[1];
                            t[i] = curT.get(i);
                        }
                        done.add(new Stroke(x, y, t));
                        cur = null;
                    }
                    break;
                default:
                    break;
            }
            invalidate();
            return true;
        }

        @Override
        protected void onDraw(Canvas cv) {
            int[] loc = new int[2];
            getLocationOnScreen(loc);
            cv.save();
            cv.translate(-loc[0], -loc[1]); // draw in screen coordinates
            int from = Math.max(0, done.size() - 25);
            for (int i = from; i < done.size(); i++) drawStroke(cv, done.get(i));
            if (cur != null && !cur.isEmpty()) {
                float[] a = cur.get(0);
                for (int i = 1; i < cur.size(); i++) {
                    float[] b = cur.get(i);
                    cv.drawLine(a[0], a[1], b[0], b[1], trail);
                    a = b;
                }
                cv.drawCircle(a[0], a[1], dp(10), dot);
            }
            cv.restore();
            cv.drawRect(0, 0, getWidth(), getHeight(), frame);
            float cx = getWidth() / 2f, y = dp(64);
            cv.drawRoundRect(cx - dp(150), y - dp(24), cx + dp(150), y + dp(12), dp(18), dp(18), hintBg);
            cv.drawText("Recording. Press the red button to finish", cx, y, hint);
        }

        private void drawStroke(Canvas cv, Stroke s) {
            if (s.maxMove() < 12f) {
                cv.drawCircle(s.x[0], s.y[0], dp(8), dot);
                return;
            }
            for (int i = 1; i < s.x.length; i++) cv.drawLine(s.x[i - 1], s.y[i - 1], s.x[i], s.y[i], trail);
        }
    }

    // ───────────────────────── tap-point mode ─────────────────────────

    private void addTap() {
        int[] s = screen();
        Step st = new Step();
        int k = steps.size() % 6;
        st.x1 = s[0] / 2f + (k - 2.5f) * dp(22);
        st.y1 = s[1] / 2f + (k % 3 - 1) * dp(40);
        steps.add(st);
        Step.saveList(this, steps);
        sounds.play(sounds.tick);
        rebuildMarkers();
        refreshButtons();
    }

    private void addSwipe() {
        if (!pointsMode) return;
        int[] s = screen();
        Step st = new Step();
        st.swipe = true;
        int k = steps.size() % 5;
        st.x1 = s[0] * 0.3f;
        st.x2 = s[0] * 0.7f;
        st.y1 = st.y2 = s[1] * 0.42f + k * dp(36);
        steps.add(st);
        Step.saveList(this, steps);
        sounds.play(sounds.tick);
        rebuildMarkers();
        refreshButtons();
    }

    private void removeLast() {
        if (steps.isEmpty()) return;
        steps.remove(steps.size() - 1);
        Step.saveList(this, steps);
        sounds.play(sounds.tick);
        rebuildMarkers();
        refreshButtons();
    }

    private void hideMarkers() {
        for (View v : markers) safeRemove(v);
        markers.clear();
        markerLps.clear();
        safeRemove(linesView);
        linesView = null;
    }

    private void rebuildMarkers() {
        hideMarkers();
        if (!pointsMode || !panelShown) return;
        linesView = new LinesView(this);
        wm.addView(linesView, lp(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT, false));
        for (int i = 0; i < steps.size(); i++) {
            addMarker(steps.get(i), i, true);
            if (steps.get(i).swipe) addMarker(steps.get(i), i, false);
        }
        bringPanelToFront();
    }

    @SuppressLint("ClickableViewAccessibility")
    private void addMarker(Step st, int idx, boolean start) {
        final int size = dp(42);
        TextView m = new TextView(this);
        m.setText(String.valueOf(idx + 1));
        m.setGravity(Gravity.CENTER);
        m.setTextSize(15);
        m.setTypeface(m.getTypeface(), android.graphics.Typeface.BOLD);
        if (start) {
            m.setTextColor(Ui.NAVY);
            m.setBackground(Ui.oval(Ui.GOLD1, Ui.GOLD2, 0xFFFFF2C8, dp(2)));
        } else {
            m.setTextColor(Ui.GOLD1);
            m.setBackground(Ui.oval(0xC0131E42, 0xC0131E42, Ui.GOLD1, dp(2.5f)));
        }
        final WindowManager.LayoutParams p = lp(size, size, true);
        p.x = (int) ((start ? st.x1 : st.x2) - size / 2f);
        p.y = (int) ((start ? st.y1 : st.y2) - size / 2f);
        m.setOnTouchListener(new View.OnTouchListener() {
            float rx, ry;
            int sx, sy;

            @Override
            public boolean onTouch(View v, MotionEvent e) {
                switch (e.getActionMasked()) {
                    case MotionEvent.ACTION_DOWN:
                        sx = p.x;
                        sy = p.y;
                        rx = e.getRawX();
                        ry = e.getRawY();
                        return true;
                    case MotionEvent.ACTION_MOVE:
                        p.x = sx + (int) (e.getRawX() - rx);
                        p.y = sy + (int) (e.getRawY() - ry);
                        try {
                            wm.updateViewLayout(v, p);
                        } catch (RuntimeException ignored) {
                        }
                        if (start) {
                            st.x1 = p.x + size / 2f;
                            st.y1 = p.y + size / 2f;
                        } else {
                            st.x2 = p.x + size / 2f;
                            st.y2 = p.y + size / 2f;
                        }
                        if (linesView != null) linesView.invalidate();
                        return true;
                    case MotionEvent.ACTION_UP:
                    case MotionEvent.ACTION_CANCEL:
                        Step.saveList(ClickerService.this, steps);
                        return true;
                    default:
                        return true;
                }
            }
        });
        wm.addView(m, p);
        markers.add(m);
        markerLps.add(p);
    }

    /** While playing, markers must let the taps through to the game. */
    private void setMarkersTouchable(boolean on) {
        for (int i = 0; i < markers.size(); i++) {
            WindowManager.LayoutParams p = markerLps.get(i);
            if (on) p.flags &= ~WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE;
            else p.flags |= WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE;
            markers.get(i).setAlpha(on ? 1f : 0.55f);
            try {
                wm.updateViewLayout(markers.get(i), p);
            } catch (RuntimeException ignored) {
            }
        }
    }

    /** Draws the arrows for swipe points. */
    private final class LinesView extends View {
        private final Paint line = new Paint(Paint.ANTI_ALIAS_FLAG);

        LinesView(Context c) {
            super(c);
            line.setColor(0xD9F6D67D);
            line.setStrokeWidth(dp(3));
            line.setStrokeCap(Paint.Cap.ROUND);
        }

        @Override
        protected void onDraw(Canvas cv) {
            int[] loc = new int[2];
            getLocationOnScreen(loc);
            cv.translate(-loc[0], -loc[1]);
            for (Step s : steps) {
                if (!s.swipe) continue;
                cv.drawLine(s.x1, s.y1, s.x2, s.y2, line);
                double ang = Math.atan2(s.y2 - s.y1, s.x2 - s.x1);
                float back = dp(30), wing = dp(10);
                float bx = (float) (s.x2 - Math.cos(ang) * back), by = (float) (s.y2 - Math.sin(ang) * back);
                float px = (float) (-Math.sin(ang) * wing), py = (float) (Math.cos(ang) * wing);
                float tx = (float) (s.x2 - Math.cos(ang) * dp(21)), ty = (float) (s.y2 - Math.sin(ang) * dp(21));
                cv.drawLine(tx, ty, bx + px, by + py, line);
                cv.drawLine(tx, ty, bx - px, by - py, line);
            }
        }
    }

    // ───────────────────────── play / stop ─────────────────────────

    private void play() {
        if (recording || player.running()) return;
        Player.Config k = Player.Config.from(this);
        if (pointsMode) {
            if (steps.isEmpty()) {
                toast("Add a tap point first with +");
                return;
            }
            setMarkersTouchable(false);
            sounds.play(sounds.play);
            player.playSteps(new ArrayList<>(steps), k);
        } else {
            if (rec == null || rec.isEmpty()) {
                toast("Record something first");
                return;
            }
            sounds.play(sounds.play);
            player.playRec(rec, k);
        }
        refreshButtons();
    }

    void stopAll() {
        if (recording) finishRecording();
        else if (player.running()) player.stop();
    }

    @Override
    public void onCount(int n) {
        if (n > 0) {
            sounds.play(sounds.count);
            showCount(String.valueOf(n));
        } else {
            sounds.play(sounds.go);
            removeCount();
        }
    }

    @Override
    public void onDone(boolean stopped) {
        removeCount();
        setMarkersTouchable(true);
        if (stopped) sounds.play(sounds.stop);
        toast(stopped ? "Stopped" : "Done!");
        refreshButtons();
    }

    private void showCount(String s) {
        if (countView == null) {
            countView = new TextView(this);
            countView.setGravity(Gravity.CENTER);
            countView.setTextSize(56);
            countView.setTextColor(Ui.GOLD1);
            countView.setTypeface(countView.getTypeface(), android.graphics.Typeface.BOLD);
            countView.setBackground(Ui.oval(0xE81E2C58, 0xE8131E42, Ui.GOLD1, dp(3)));
            WindowManager.LayoutParams p = lp(dp(132), dp(132), false);
            p.gravity = Gravity.CENTER;
            wm.addView(countView, p);
        }
        countView.setText(s);
    }

    private void removeCount() {
        safeRemove(countView);
        countView = null;
    }
}
