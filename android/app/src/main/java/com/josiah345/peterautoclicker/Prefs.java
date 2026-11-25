package com.josiah345.peterautoclicker;

import android.content.Context;
import android.content.SharedPreferences;

/** Saved settings (they survive closing the app). */
final class Prefs {
    private Prefs() {}

    private static SharedPreferences sp(Context c) {
        return c.getSharedPreferences("settings", Context.MODE_PRIVATE);
    }

    static int getInt(Context c, String k, int def) { return sp(c).getInt(k, def); }
    static void putInt(Context c, String k, int v) { sp(c).edit().putInt(k, v).apply(); }
    static float getFloat(Context c, String k, float def) { return sp(c).getFloat(k, def); }
    static void putFloat(Context c, String k, float v) { sp(c).edit().putFloat(k, v).apply(); }
    static boolean getBool(Context c, String k, boolean def) { return sp(c).getBoolean(k, def); }
    static void putBool(Context c, String k, boolean v) { sp(c).edit().putBoolean(k, v).apply(); }
    static String getStr(Context c, String k) { return sp(c).getString(k, null); }
    static void putStr(Context c, String k, String v) { sp(c).edit().putString(k, v).apply(); }

    // keys and defaults
    static final String REPEAT = "repeat";        // 10
    static final String FOREVER = "forever";      // false
    static final String PAUSE = "pauseMs";        // 0
    static final String SPEED = "speed";          // 1.0
    static final String SMART = "smart";          // false
    static final String INTERVAL = "intervalMs";  // 500
    static final String COUNTDOWN = "countdown";  // true
    static final String SOUNDS = "sounds";        // true
    static final String POINTS_MODE = "pointsMode";
    static final String REC = "recording";
    static final String STEPS = "steps";
}
