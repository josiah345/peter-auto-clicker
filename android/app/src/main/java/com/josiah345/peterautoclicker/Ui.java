package com.josiah345.peterautoclicker;

import android.content.Context;
import android.content.res.ColorStateList;
import android.graphics.PorterDuff;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.widget.ImageView;
import android.widget.TextView;

/** Colours and small styling helpers for the navy and gold look. */
final class Ui {
    private Ui() {}

    static final int BG1 = 0xFF101C3E, BG2 = 0xFF060A1A;
    static final int CARD1 = 0xFF1E2C58, CARD2 = 0xFF131E42, BORDER = 0x96D6AC52;
    static final int FIELD = 0xFF090F24, FIELD_B = 0xFF465682;
    static final int TEXT = 0xFFF3E9CC, MUTED = 0xFF96A6CC, ICON = 0xFFE8D6A5;
    static final int GOLD1 = 0xFFF6D67D, GOLD2 = 0xFFC4922E, NAVY = 0xFF141C38;
    static final int RED1 = 0xFFE85050, RED2 = 0xFF961C2C;
    static final int GREEN1 = 0xFF34C878, GREEN2 = 0xFF107846;
    static final int SLATE1 = 0xFF5A6A96, SLATE2 = 0xFF2C3658;
    static final int OK = 0xFF2ECC71, WARN = 0xFFF5A623;

    static int dp(Context c, float v) {
        return Math.round(v * c.getResources().getDisplayMetrics().density);
    }

    static GradientDrawable grad(int c1, int c2, float radiusPx, int strokeColor, int strokePx) {
        GradientDrawable g = new GradientDrawable(GradientDrawable.Orientation.TOP_BOTTOM, new int[]{c1, c2});
        g.setCornerRadius(radiusPx);
        if (strokePx > 0) g.setStroke(strokePx, strokeColor);
        return g;
    }

    static GradientDrawable oval(int c1, int c2, int strokeColor, int strokePx) {
        GradientDrawable g = new GradientDrawable(GradientDrawable.Orientation.TOP_BOTTOM, new int[]{c1, c2});
        g.setShape(GradientDrawable.OVAL);
        if (strokePx > 0) g.setStroke(strokePx, strokeColor);
        return g;
    }

    static GradientDrawable card(Context c) {
        return grad(CARD1, CARD2, dp(c, 14), BORDER, dp(c, 1.5f));
    }

    static GradientDrawable field(Context c) {
        return grad(FIELD, FIELD, dp(c, 10), FIELD_B, dp(c, 1));
    }

    static GradientDrawable gold(Context c, float radiusDp) {
        return grad(GOLD1, GOLD2, dp(c, radiusDp), 0xFFFFECAA, dp(c, 1.5f));
    }

    static TextView text(Context c, String s, float sp, int color, boolean bold) {
        TextView t = new TextView(c);
        t.setText(s);
        t.setTextSize(sp);
        t.setTextColor(color);
        if (bold) t.setTypeface(Typeface.create("sans-serif-medium", Typeface.BOLD));
        return t;
    }

    static TextView caption(Context c, String s) {
        TextView t = text(c, s, 11, MUTED, true);
        t.setLetterSpacing(0.12f);
        return t;
    }

    static void tint(ImageView v, int color) {
        v.setColorFilter(color, PorterDuff.Mode.SRC_IN);
    }

    static ColorStateList states(int checked, int normal) {
        return new ColorStateList(new int[][]{{android.R.attr.state_checked}, {}}, new int[]{checked, normal});
    }
}
