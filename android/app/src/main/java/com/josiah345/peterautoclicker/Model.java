package com.josiah345.peterautoclicker;

import android.content.Context;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.List;

/** One finger stroke: points with times (ms since the recording started). */
final class Stroke {
    final float[] x, y;
    final long[] t;

    Stroke(float[] x, float[] y, long[] t) {
        this.x = x;
        this.y = y;
        this.t = t;
    }

    long start() { return t[0]; }
    long end() { return t[t.length - 1]; }

    /** Largest distance from the first point, in px. */
    float maxMove() {
        float m = 0;
        for (int i = 1; i < x.length; i++) {
            float dx = x[i] - x[0], dy = y[i] - y[0];
            m = Math.max(m, (float) Math.sqrt(dx * dx + dy * dy));
        }
        return m;
    }
}

/** A recorded session. */
final class Rec {
    long duration;
    final ArrayList<Stroke> strokes = new ArrayList<>();

    boolean isEmpty() { return strokes.isEmpty(); }

    void save(Context c) {
        try {
            JSONObject o = new JSONObject();
            o.put("duration", duration);
            JSONArray arr = new JSONArray();
            for (Stroke s : strokes) {
                JSONArray pts = new JSONArray();
                for (int i = 0; i < s.x.length; i++) {
                    pts.put(new JSONArray().put((double) s.x[i]).put((double) s.y[i]).put(s.t[i]));
                }
                arr.put(pts);
            }
            o.put("strokes", arr);
            Prefs.putStr(c, Prefs.REC, o.toString());
        } catch (JSONException ignored) {
        }
    }

    static Rec load(Context c) {
        Rec r = new Rec();
        String s = Prefs.getStr(c, Prefs.REC);
        if (s == null) return r;
        try {
            JSONObject o = new JSONObject(s);
            r.duration = o.getLong("duration");
            JSONArray arr = o.getJSONArray("strokes");
            for (int i = 0; i < arr.length(); i++) {
                JSONArray pts = arr.getJSONArray(i);
                int n = pts.length();
                float[] x = new float[n], y = new float[n];
                long[] t = new long[n];
                for (int j = 0; j < n; j++) {
                    JSONArray p = pts.getJSONArray(j);
                    x[j] = (float) p.getDouble(0);
                    y[j] = (float) p.getDouble(1);
                    t[j] = p.getLong(2);
                }
                if (n > 0) r.strokes.add(new Stroke(x, y, t));
            }
        } catch (JSONException e) {
            r.strokes.clear();
        }
        return r;
    }
}

/** A placed tap point or swipe (tap-point mode). */
final class Step {
    boolean swipe;
    float x1, y1, x2, y2;

    static void saveList(Context c, List<Step> list) {
        try {
            JSONArray arr = new JSONArray();
            for (Step s : list) {
                arr.put(new JSONArray().put(s.swipe ? 1 : 0).put((double) s.x1).put((double) s.y1).put((double) s.x2).put((double) s.y2));
            }
            Prefs.putStr(c, Prefs.STEPS, arr.toString());
        } catch (JSONException ignored) {
        }
    }

    static ArrayList<Step> loadList(Context c) {
        ArrayList<Step> out = new ArrayList<>();
        String s = Prefs.getStr(c, Prefs.STEPS);
        if (s == null) return out;
        try {
            JSONArray arr = new JSONArray(s);
            for (int i = 0; i < arr.length(); i++) {
                JSONArray a = arr.getJSONArray(i);
                Step st = new Step();
                st.swipe = a.getInt(0) == 1;
                st.x1 = (float) a.getDouble(1);
                st.y1 = (float) a.getDouble(2);
                st.x2 = (float) a.getDouble(3);
                st.y2 = (float) a.getDouble(4);
                out.add(st);
            }
        } catch (JSONException e) {
            out.clear();
        }
        return out;
    }
}
