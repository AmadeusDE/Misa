package app.misa;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;

// Foreground service that owns the Go library's process lifetime (D11).
// Switching apps destroys the surface; the sessions keep running until Stop
// or the last one exits.
public final class SessionService extends Service {
    public static final String ACTION_STOP = "app.misa.action.STOP";
    public static final String ACTION_EXIT = "app.misa.action.EXIT";

    private static final int ID = 1;
    private static final String CHANNEL = "sessions";
    private static final long REFRESH_MS = 1000;

    private final Handler handler = new Handler(Looper.getMainLooper());
    private boolean refreshArmed;

    private final Runnable refresh = new Runnable() {
        @Override
        public void run() {
            refreshArmed = false;
            int sessions = Native.sessionCount();
            if (sessions <= 0) {
                halt(false);
                return;
            }
            startInForeground(sessions);
            arm();
        }
    };

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    @Override
    public void onCreate() {
        super.onCreate();
        NotificationChannel channel =
                new NotificationChannel(CHANNEL, getString(R.string.notif_channel), NotificationManager.IMPORTANCE_LOW);
        channel.setShowBadge(false);
        NotificationManager manager = getSystemService(NotificationManager.class);
        if (manager != null) {
            manager.createNotificationChannel(channel);
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        Bootstrap.ensureNative(this);
        startInForeground(Native.sessionCount());
        if (intent != null && ACTION_STOP.equals(intent.getAction())) {
            closeAll();
            halt(true);
            return START_NOT_STICKY;
        }
        arm();
        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        handler.removeCallbacks(refresh);
        refreshArmed = false;
        super.onDestroy();
    }

    private void arm() {
        if (refreshArmed) {
            return;
        }
        refreshArmed = true;
        handler.postDelayed(refresh, REFRESH_MS);
    }

    private void closeAll() {
        for (int i = 0; i < 64 && Native.sessionCount() > 0; i++) {
            int id = Native.activeSession();
            if (id == 0) {
                break;
            }
            Native.closeSession(id);
        }
    }

    private void halt(boolean exitActivity) {
        handler.removeCallbacks(refresh);
        refreshArmed = false;
        if (exitActivity) {
            sendBroadcast(new Intent(ACTION_EXIT).setPackage(getPackageName()));
        }
        stopForeground(STOP_FOREGROUND_REMOVE);
        stopSelf();
    }

    private void startInForeground(int sessions) {
        Notification notification = notification(sessions);
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE);
        } else {
            startForeground(ID, notification);
        }
    }

    private Notification notification(int sessions) {
        Intent open = new Intent(this, MisaActivity.class);
        open.setFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_NEW_TASK);
        PendingIntent content = PendingIntent.getActivity(
                this, 0, open, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        Intent stop = new Intent(this, SessionService.class).setAction(ACTION_STOP);
        PendingIntent stopAction = PendingIntent.getService(
                this, 1, stop, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        String text = sessions == 1
                ? getString(R.string.notif_one)
                : getString(R.string.notif_many, sessions);
        return new Notification.Builder(this, CHANNEL)
                .setSmallIcon(R.drawable.ic_stat)
                .setContentTitle(getString(R.string.app_name))
                .setContentText(text)
                .setOngoing(true)
                .setOnlyAlertOnce(true)
                .setCategory(Notification.CATEGORY_SERVICE)
                .setContentIntent(content)
                .addAction(new Notification.Action.Builder(
                                R.drawable.ic_stat, getString(R.string.stop), stopAction)
                        .build())
                .build();
    }
}
