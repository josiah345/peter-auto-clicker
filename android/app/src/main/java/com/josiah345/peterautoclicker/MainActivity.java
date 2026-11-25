package com.josiah345.peterautoclicker;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Outline;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Bundle;
import android.provider.Settings;
import android.text.Editable;
import android.text.InputType;
import android.text.TextWatcher;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.view.ViewOutlineProvider;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.Switch;
import android.widget.TextView;

import java.util.Locale;

/** Main screen: one-time setup, the button that opens the floating controls, and settings. */
public class MainActivity extends Activity {

    private static final String VERSION = "1.0";
    private static final String REPO = "https://github.com/josiah345/peter-auto-clicker";

    private TextView serviceState;
    private View serviceDot;
    private TextView openBtn;
    private TextView precise, smart;
    private EditText repeatEdit;

    private int dp(float v) { return Ui.dp(this, v); }

    @Override
    protected void onCreate(Bundle b) {
        super.onCreate(b);
        getWindow().setStatusBarColor(0xFF0A122C);
        getWindow().setNavigationBarColor(Ui.BG2);

        ScrollView scroll = new ScrollView(this);
        scroll.setBackground(new GradientDrawable(GradientDrawable.Orientation.TOP_BOTTOM, new int[]{Ui.BG1, Ui.BG2}));
        scroll.setFillViewport(true);
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setPadding(dp(16), 0, dp(16), dp(24));
        scroll.addView(root);

        root.addView(header());

        // ── setup
        LinearLayout setup = card(root, "SETUP");
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        serviceDot = new View(this);
        LinearLayout.LayoutParams dl = new LinearLayout.LayoutParams(dp(10), dp(10));
        dl.rightMargin = dp(10);
        row.addView(serviceDot, dl);
        serviceState = Ui.text(this, "", 15, Ui.TEXT, true);
        row.addView(serviceState);
        setup.addView(row);
        TextView a11y = button("Turn on in Accessibility settings", false);
        a11y.setOnClickListener(v -> startActivity(new Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)));
        setup.addView(a11y, spaced(10));
        TextView help = Ui.text(this,
                "Open Accessibility, tap Peter Auto Clicker (it may be under Downloaded apps) and switch it on.\n\n"
                        + "Android 13 and newer: if the switch is greyed out, go to Settings, Apps, Peter Auto Clicker, "
                        + "tap the three dots at the top right, choose Allow restricted settings, then try again.",
                12.5f, Ui.MUTED, false);
        setup.addView(help, spaced(10));

        openBtn = button("Open floating controls", true);
        openBtn.setOnClickListener(v -> openControls());
        LinearLayout.LayoutParams ol = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(56));
        ol.topMargin = dp(16);
        root.addView(openBtn, ol);

        // ── playback
        LinearLayout pb = card(root, "PLAYBACK");
        repeatEdit = numberField(Prefs.getInt(this, Prefs.REPEAT, 10) + "", false, s -> Prefs.putInt(this, Prefs.REPEAT, parseInt(s, 1)));
        LinearLayout repeatRow = settingRow(pb, R.drawable.ic_repeat, "Repeat (times)", repeatEdit, null);
        repeatRow.setPadding(0, 0, 0, 0);
        Switch forever = goldSwitch(Prefs.getBool(this, Prefs.FOREVER, false));
        forever.setOnCheckedChangeListener((v, on) -> {
            Prefs.putBool(this, Prefs.FOREVER, on);
            repeatEdit.setEnabled(!on);
            repeatEdit.setAlpha(on ? 0.4f : 1f);
        });
        repeatEdit.setEnabled(!forever.isChecked());
        repeatEdit.setAlpha(forever.isChecked() ? 0.4f : 1f);
        settingRow(pb, R.drawable.ic_inf, "Loop forever", null, forever);
        settingRow(pb, R.drawable.ic_gap, "Pause between loops (ms)",
                numberField(Prefs.getInt(this, Prefs.PAUSE, 0) + "", false, s -> Prefs.putInt(this, Prefs.PAUSE, parseInt(s, 0))), null);
        settingRow(pb, R.drawable.ic_speed, "Speed (1 = real pace)",
                numberField(fmt(Prefs.getFloat(this, Prefs.SPEED, 1f)), true, s -> Prefs.putFloat(this, Prefs.SPEED, parseFloat(s))), null);
        settingRow(pb, R.drawable.ic_tap, "Tap points: time between taps (ms)",
                numberField(Prefs.getInt(this, Prefs.INTERVAL, 500) + "", false, s -> Prefs.putInt(this, Prefs.INTERVAL, parseInt(s, 0))), null);

        // ── mode
        LinearLayout mode = card(root, "MODE");
        LinearLayout seg = new LinearLayout(this);
        seg.setBackground(Ui.field(this));
        seg.setPadding(dp(4), dp(4), dp(4), dp(4));
        precise = segment("Precise");
        smart = segment("Smart");
        seg.addView(precise, new LinearLayout.LayoutParams(0, dp(42), 1));
        LinearLayout.LayoutParams sl = new LinearLayout.LayoutParams(0, dp(42), 1);
        sl.leftMargin = dp(4);
        seg.addView(smart, sl);
        precise.setOnClickListener(v -> setSmart(false));
        smart.setOnClickListener(v -> setSmart(true));
        mode.addView(seg);
        mode.addView(Ui.text(this, "Precise replays every movement at the exact pace. Smart is for slower phones: "
                + "it sends fewer, longer pieces of each swipe so it keeps up.", 12.5f, Ui.MUTED, false), spaced(10));
        setSmart(Prefs.getBool(this, Prefs.SMART, false));

        // ── options
        LinearLayout opt = card(root, "OPTIONS");
        Switch count = goldSwitch(Prefs.getBool(this, Prefs.COUNTDOWN, true));
        count.setOnCheckedChangeListener((v, on) -> Prefs.putBool(this, Prefs.COUNTDOWN, on));
        settingRow(opt, R.drawable.ic_timer, "Countdown (3 seconds)", null, count);
        Switch snd = goldSwitch(Prefs.getBool(this, Prefs.SOUNDS, true));
        snd.setOnCheckedChangeListener((v, on) -> Prefs.putBool(this, Prefs.SOUNDS, on));
        settingRow(opt, R.drawable.ic_vol, "Click sounds", null, snd);

        // ── how to
        LinearLayout how = card(root, "HOW IT WORKS");
        how.addView(Ui.text(this,
                "Record mode: press the red button on the panel, then do your taps and swipes on the screen. "
                        + "The game doesn't react while you record. Press the red button again to finish, then Play.\n\n"
                        + "Tap-point mode: switch with the top button on the panel. + adds a tap point, the arrows add a swipe. "
                        + "Drag the numbered circles to where they should tap.\n\n"
                        + "To stop: press Stop on the panel, or Volume Down.\n\n"
                        + "Tip: drag the panel away from the spots it taps.",
                13.5f, Ui.TEXT, false));

        // ── footer
        TextView foot = Ui.text(this, "v" + VERSION + "   ·   Official page on GitHub", 12.5f, Ui.GOLD1, true);
        foot.setGravity(Gravity.CENTER);
        foot.setPadding(0, dp(20), 0, dp(4));
        foot.setOnClickListener(v -> startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(REPO))));
        root.addView(foot);
        TextView note = Ui.text(this, "Some online games don't allow auto clickers. Check a game's rules first.", 11.5f, Ui.MUTED, false);
        note.setGravity(Gravity.CENTER);
        root.addView(note);

        setContentView(scroll);
    }

    @Override
    protected void onResume() {
        super.onResume();
        boolean on = ClickerService.instance != null;
        serviceState.setText(on ? "Accessibility service is on" : "Accessibility service is off");
        GradientDrawable d = new GradientDrawable();
        d.setShape(GradientDrawable.OVAL);
        d.setColor(on ? Ui.OK : Ui.WARN);
        serviceDot.setBackground(d);
        openBtn.setAlpha(on ? 1f : 0.45f);
    }

    private void openControls() {
        ClickerService s = ClickerService.instance;
        if (s == null) {
            startActivity(new Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS));
            return;
        }
        s.showPanel();
        moveTaskToBack(true); // go to the home screen so you can open your game
    }

    // ───────────────────────── building blocks ─────────────────────────

    private View header() {
        LinearLayout h = new LinearLayout(this);
        h.setGravity(Gravity.CENTER_VERTICAL);
        h.setPadding(0, dp(28), 0, dp(8));

        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(Ui.gold(this, 18));
        frame.setPadding(dp(3), dp(3), dp(3), dp(3));
        ImageView logo = new ImageView(this);
        logo.setImageResource(R.drawable.logo);
        logo.setScaleType(ImageView.ScaleType.CENTER_CROP);
        logo.setOutlineProvider(new ViewOutlineProvider() {
            @Override
            public void getOutline(View v, Outline o) {
                o.setRoundRect(0, 0, v.getWidth(), v.getHeight(), dp(15));
            }
        });
        logo.setClipToOutline(true);
        frame.addView(logo, new FrameLayout.LayoutParams(dp(72), dp(72)));
        h.addView(frame);

        LinearLayout t = new LinearLayout(this);
        t.setOrientation(LinearLayout.VERTICAL);
        t.setPadding(dp(16), 0, 0, 0);
        TextView title = Ui.text(this, "Peter Auto Clicker", 24, Ui.GOLD1, true);
        title.setShadowLayer(dp(2), 0, dp(1), 0xFF000000);
        t.addView(title);
        TextView sub = Ui.text(this, "RECORD  ·  REPLAY  ·  REPEAT", 11, 0xFFAABAE4, true);
        sub.setLetterSpacing(0.1f);
        t.addView(sub);
        h.addView(t);
        return h;
    }

    private LinearLayout card(LinearLayout root, String caption) {
        LinearLayout c = new LinearLayout(this);
        c.setOrientation(LinearLayout.VERTICAL);
        c.setBackground(Ui.card(this));
        c.setPadding(dp(16), dp(12), dp(16), dp(16));
        c.addView(Ui.caption(this, caption), new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT));
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        lp.topMargin = dp(14);
        root.addView(c, lp);
        return c;
    }

    private LinearLayout.LayoutParams spaced(int topDp) {
        LinearLayout.LayoutParams p = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        p.topMargin = dp(topDp);
        return p;
    }

    private TextView button(String label, boolean gold) {
        TextView b = Ui.text(this, label, gold ? 17 : 14.5f, gold ? Ui.NAVY : Ui.ICON, true);
        b.setGravity(Gravity.CENTER);
        b.setPadding(dp(12), dp(12), dp(12), dp(12));
        b.setBackground(gold ? Ui.gold(this, 16) : Ui.grad(Ui.CARD1, Ui.CARD2, dp(12), Ui.GOLD1, dp(1.5f)));
        b.setClickable(true);
        return b;
    }

    private TextView segment(String label) {
        TextView t = Ui.text(this, label, 15, Ui.MUTED, true);
        t.setGravity(Gravity.CENTER);
        t.setClickable(true);
        return t;
    }

    private void setSmart(boolean on) {
        Prefs.putBool(this, Prefs.SMART, on);
        TextView sel = on ? smart : precise, other = on ? precise : smart;
        sel.setBackground(Ui.gold(this, 9));
        sel.setTextColor(Ui.NAVY);
        other.setBackground(null);
        other.setTextColor(Ui.MUTED);
    }

    private LinearLayout settingRow(LinearLayout card, int icon, String label, EditText field, Switch sw) {
        LinearLayout r = new LinearLayout(this);
        r.setGravity(Gravity.CENTER_VERTICAL);
        r.setPadding(0, dp(10), 0, 0);
        ImageView ic = new ImageView(this);
        ic.setImageResource(icon);
        Ui.tint(ic, Ui.ICON);
        r.addView(ic, new LinearLayout.LayoutParams(dp(22), dp(22)));
        TextView t = Ui.text(this, label, 14.5f, Ui.TEXT, false);
        LinearLayout.LayoutParams tl = new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1);
        tl.leftMargin = dp(12);
        r.addView(t, tl);
        if (field != null) r.addView(field, new LinearLayout.LayoutParams(dp(86), dp(42)));
        if (sw != null) r.addView(sw);
        card.addView(r);
        return r;
    }

    private interface OnText {
        void changed(String s);
    }

    private EditText numberField(String value, boolean decimal, OnText cb) {
        EditText e = new EditText(this);
        e.setText(value);
        e.setTextColor(Ui.TEXT);
        e.setTextSize(15);
        e.setGravity(Gravity.CENTER);
        e.setSingleLine(true);
        e.setBackground(Ui.field(this));
        e.setPadding(dp(8), 0, dp(8), 0);
        e.setInputType(decimal ? InputType.TYPE_CLASS_NUMBER | InputType.TYPE_NUMBER_FLAG_DECIMAL : InputType.TYPE_CLASS_NUMBER);
        e.addTextChangedListener(new TextWatcher() {
            @Override
            public void beforeTextChanged(CharSequence s, int a, int b, int c) {
            }

            @Override
            public void onTextChanged(CharSequence s, int a, int b, int c) {
            }

            @Override
            public void afterTextChanged(Editable s) {
                cb.changed(s.toString());
            }
        });
        return e;
    }

    private Switch goldSwitch(boolean on) {
        Switch s = new Switch(this);
        s.setChecked(on);
        s.setThumbTintList(Ui.states(Ui.GOLD1, 0xFFB0B8D0));
        s.setTrackTintList(Ui.states(Ui.GOLD2, 0xFF465682));
        return s;
    }

    private static int parseInt(String s, int min) {
        try {
            return Math.max(min, Integer.parseInt(s.trim()));
        } catch (NumberFormatException e) {
            return min;
        }
    }

    private static float parseFloat(String s) {
        try {
            float f = Float.parseFloat(s.trim().replace(',', '.'));
            return Math.max(0.1f, Math.min(10f, f));
        } catch (NumberFormatException e) {
            return 1f;
        }
    }

    private static String fmt(float f) {
        if (f == Math.round(f)) return String.valueOf(Math.round(f));
        return String.format(Locale.US, "%.2f", f);
    }
}
