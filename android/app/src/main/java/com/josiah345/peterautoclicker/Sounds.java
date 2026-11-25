package com.josiah345.peterautoclicker;

import android.content.Context;
import android.media.AudioAttributes;
import android.media.SoundPool;

/** Short UI sounds (our own synthesized tones in res/raw). */
final class Sounds {
    private final Context ctx;
    private final SoundPool pool;
    final int tick, toggle, rec, finish, play, stop, count, go;

    Sounds(Context c) {
        ctx = c;
        pool = new SoundPool.Builder()
                .setMaxStreams(4)
                .setAudioAttributes(new AudioAttributes.Builder()
                        .setUsage(AudioAttributes.USAGE_ASSISTANCE_SONIFICATION)
                        .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
                        .build())
                .build();
        tick = pool.load(c, R.raw.snd_tick, 1);
        toggle = pool.load(c, R.raw.snd_toggle, 1);
        rec = pool.load(c, R.raw.snd_rec, 1);
        finish = pool.load(c, R.raw.snd_finish, 1);
        play = pool.load(c, R.raw.snd_play, 1);
        stop = pool.load(c, R.raw.snd_stop, 1);
        count = pool.load(c, R.raw.snd_count, 1);
        go = pool.load(c, R.raw.snd_go, 1);
    }

    void play(int id) {
        if (Prefs.getBool(ctx, Prefs.SOUNDS, true)) pool.play(id, 1f, 1f, 1, 0, 1f);
    }

    void release() { pool.release(); }
}
