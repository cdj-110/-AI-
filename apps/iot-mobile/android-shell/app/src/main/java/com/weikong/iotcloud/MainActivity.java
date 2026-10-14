package com.weikong.iotcloud;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.Manifest;
import android.content.Intent;
import android.content.Context;
import android.content.pm.PackageManager;
import android.graphics.Color;
import android.net.Uri;
import android.nfc.NdefMessage;
import android.nfc.NdefRecord;
import android.nfc.NfcAdapter;
import android.nfc.Tag;
import android.nfc.tech.Ndef;
import android.nfc.tech.NdefFormatable;
import android.os.Build;
import android.os.Bundle;
import android.os.VibrationEffect;
import android.os.Vibrator;
import android.os.VibratorManager;
import android.view.View;
import android.view.ViewGroup;
import android.webkit.JavascriptInterface;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;
import androidx.core.graphics.Insets;
import androidx.core.app.NotificationCompat;
import androidx.core.view.ViewCompat;
import androidx.core.view.WindowInsetsCompat;
import org.json.JSONObject;
import java.util.concurrent.atomic.AtomicBoolean;

import com.google.zxing.integration.android.IntentIntegrator;
import com.google.zxing.integration.android.IntentResult;

public class MainActivity extends Activity {
    private static final int CAMERA_PERMISSION_REQUEST = 2001;
    private static final int NOTIFICATION_PERMISSION_REQUEST = 2002;
    private static final String ALARM_CHANNEL_ID = "weikong_device_alarms";
    private WebView webView;
    private boolean scanPending;
    private NfcAdapter nfcAdapter;
    private volatile String nfcWritePayload;
    private volatile boolean nfcWriteSessionActive;
    private volatile Tag pendingNfcWriteTag;
    private final AtomicBoolean nfcWriteInProgress = new AtomicBoolean(false);
    private volatile String pendingNfcPayload;
    private volatile boolean pendingOpenAlarms;

    @SuppressLint("SetJavaScriptEnabled")
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        getWindow().setStatusBarColor(Color.rgb(255, 255, 255));
        getWindow().setNavigationBarColor(Color.rgb(255, 255, 255));
        getWindow().getDecorView().setSystemUiVisibility(
            View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR | View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR
        );

        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(Color.WHITE);
        webView = new WebView(this);
        FrameLayout.LayoutParams webViewLayout = new FrameLayout.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT,
            ViewGroup.LayoutParams.MATCH_PARENT
        );
        root.addView(webView, webViewLayout);
        ViewCompat.setOnApplyWindowInsetsListener(root, (view, windowInsets) -> {
            Insets statusBars = windowInsets.getInsets(WindowInsetsCompat.Type.statusBars());
            FrameLayout.LayoutParams layout = (FrameLayout.LayoutParams) webView.getLayoutParams();
            if (layout.topMargin != statusBars.top) {
                layout.topMargin = statusBars.top;
                webView.setLayoutParams(layout);
            }
            return windowInsets;
        });
        setContentView(root);
        ViewCompat.requestApplyInsets(root);
        nfcAdapter = NfcAdapter.getDefaultAdapter(this);
        createAlarmNotificationChannel();
        pendingOpenAlarms = "com.weikong.iotcloud.OPEN_ALARMS".equals(getIntent().getAction());
        handleNfcIntent(getIntent());

        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setDatabaseEnabled(true);
        settings.setAllowFileAccess(true);
        settings.setAllowContentAccess(true);
        settings.setAllowFileAccessFromFileURLs(true);
        settings.setAllowUniversalAccessFromFileURLs(true);
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        settings.setCacheMode(WebSettings.LOAD_DEFAULT);
        settings.setMediaPlaybackRequiresUserGesture(false);
        settings.setUserAgentString(settings.getUserAgentString() + " WeikongIoTCloud/0.1");

        webView.setWebViewClient(new WebViewClient() {
            @Override
            public void onPageFinished(WebView view, String url) {
                super.onPageFinished(view, url);
                if (pendingOpenAlarms) {
                    pendingOpenAlarms = false;
                    callJavaScript("window.__weikongOpenAlarms", "");
                }
            }
        });
        webView.setWebChromeClient(new WebChromeClient());
        webView.addJavascriptInterface(new ScannerBridge(), "WeikongNative");
        webView.loadUrl("file:///android_asset/www/index.html");
    }

    private final class ScannerBridge {
        @JavascriptInterface
        public String portalMode() {
            return BuildConfig.FLAVOR;
        }

        @JavascriptInterface
        public void scanQRCode() {
            runOnUiThread(MainActivity.this::prepareQRCodeScan);
        }

        @JavascriptInterface
        public boolean isNfcAvailable() {
            return nfcAdapter != null && nfcAdapter.isEnabled();
        }

        @JavascriptInterface
        public void prepareGatewayNfcWrite() {
            runOnUiThread(MainActivity.this::prepareNfcWriteSession);
        }

        @JavascriptInterface
        public void writeGatewayNfc(String payload) {
            runOnUiThread(() -> startNfcWrite(payload));
        }

        @JavascriptInterface
        public void cancelNfcWrite() {
            runOnUiThread(MainActivity.this::stopNfcWrite);
        }

        @JavascriptInterface
        public String consumeGatewayNfc() {
            String payload = pendingNfcPayload;
            pendingNfcPayload = null;
            return payload == null ? "" : payload;
        }

        @JavascriptInterface
        public void hapticFeedback(String type) {
            runOnUiThread(() -> vibrateFeedback(type));
        }

        @JavascriptInterface
        public void requestNotificationPermission() {
            runOnUiThread(MainActivity.this::requestAlarmNotificationPermission);
        }

        @JavascriptInterface
        public void showAlarmNotification(String id, String title, String message) {
            runOnUiThread(() -> showAlarmNotificationNative(id, title, message));
        }
    }

    private void createAlarmNotificationChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return;
        NotificationChannel channel = new NotificationChannel(
            ALARM_CHANNEL_ID,
            "设备告警",
            NotificationManager.IMPORTANCE_HIGH
        );
        channel.setDescription("接收物联云设备异常告警");
        channel.enableVibration(true);
        channel.setVibrationPattern(new long[]{0, 180, 100, 220});
        NotificationManager manager = getSystemService(NotificationManager.class);
        if (manager != null) manager.createNotificationChannel(channel);
    }

    private void requestAlarmNotificationPermission() {
        if (Build.VERSION.SDK_INT >= 33
            && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, NOTIFICATION_PERMISSION_REQUEST);
        }
    }

    private void showAlarmNotificationNative(String id, String title, String message) {
        if (BuildConfig.FLAVOR.equals("admin")) return;
        if (Build.VERSION.SDK_INT >= 33
            && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) return;
        Intent launch = new Intent(this, MainActivity.class)
            .setAction("com.weikong.iotcloud.OPEN_ALARMS")
            .addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP | Intent.FLAG_ACTIVITY_SINGLE_TOP);
        PendingIntent pendingIntent = PendingIntent.getActivity(
            this,
            id == null ? 0 : id.hashCode(),
            launch,
            PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE
        );
        Notification notification = new NotificationCompat.Builder(this, ALARM_CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_alarm_notification)
            .setContentTitle(title == null || title.isEmpty() ? "设备告警" : title)
            .setContentText(message)
            .setStyle(new NotificationCompat.BigTextStyle().bigText(message))
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setCategory(NotificationCompat.CATEGORY_ALARM)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .setVibrate(new long[]{0, 180, 100, 220})
            .build();
        NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager != null) manager.notify(id == null ? (int) System.currentTimeMillis() : id.hashCode(), notification);
    }

    private void vibrateFeedback(String type) {
        long[] pattern = "error".equalsIgnoreCase(type)
            ? new long[]{0, 85, 65, 85, 65, 125}
            : new long[]{0, 45, 55, 75};
        Vibrator vibrator;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            VibratorManager manager = (VibratorManager) getSystemService(Context.VIBRATOR_MANAGER_SERVICE);
            vibrator = manager == null ? null : manager.getDefaultVibrator();
        } else {
            vibrator = (Vibrator) getSystemService(Context.VIBRATOR_SERVICE);
        }
        if (vibrator == null || !vibrator.hasVibrator()) return;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            vibrator.vibrate(VibrationEffect.createWaveform(pattern, -1));
        } else {
            vibrator.vibrate(pattern, -1);
        }
    }

    private void prepareNfcWriteSession() {
        if (nfcAdapter == null || !nfcAdapter.isEnabled()) return;
        nfcWritePayload = null;
        pendingNfcWriteTag = null;
        nfcWriteInProgress.set(false);
        nfcWriteSessionActive = true;
        enableNfcReaderMode();
    }

    private void startNfcWrite(String payload) {
        if (nfcAdapter == null) {
            callJavaScript("window.__weikongNfcWriteError", "当前手机不支持 NFC");
            return;
        }
        if (!nfcAdapter.isEnabled()) {
            callJavaScript("window.__weikongNfcWriteError", "请先在系统设置中开启 NFC");
            return;
        }
        if (payload == null || !payload.startsWith("weikong://gateway-bind?")) {
            callJavaScript("window.__weikongNfcWriteError", "待写入的网关凭证格式无效");
            return;
        }
        nfcWriteSessionActive = true;
        nfcWritePayload = payload;
        Tag alreadyPresentTag = pendingNfcWriteTag;
        pendingNfcWriteTag = null;
        enableNfcReaderMode();
        if (alreadyPresentTag != null) {
            new Thread(() -> writeNfcTag(alreadyPresentTag), "nfc-write").start();
        }
    }

    private void enableNfcReaderMode() {
        if (nfcAdapter == null || !nfcWriteSessionActive) return;
        int flags = NfcAdapter.FLAG_READER_NFC_A | NfcAdapter.FLAG_READER_NFC_B
            | NfcAdapter.FLAG_READER_NFC_F | NfcAdapter.FLAG_READER_NFC_V
            | NfcAdapter.FLAG_READER_NO_PLATFORM_SOUNDS;
        nfcAdapter.enableReaderMode(this, this::writeNfcTag, flags, null);
    }

    private void writeNfcTag(Tag tag) {
        String payload = nfcWritePayload;
        if (payload == null) {
            pendingNfcWriteTag = tag;
            return;
        }
        if (!nfcWriteInProgress.compareAndSet(false, true)) return;
        NdefMessage message = new NdefMessage(new NdefRecord[]{NdefRecord.createUri(Uri.parse(payload))});
        try {
            Ndef ndef = Ndef.get(tag);
            if (ndef != null) {
                ndef.connect();
                try {
                    if (!ndef.isWritable()) throw new IllegalStateException("该 NFC 标签已锁定，无法写入");
                    if (ndef.getMaxSize() < message.toByteArray().length) throw new IllegalStateException("标签容量不足，请更换容量更大的 NFC 标签");
                    ndef.writeNdefMessage(message);
                } finally {
                    ndef.close();
                }
            } else {
                NdefFormatable formatable = NdefFormatable.get(tag);
                if (formatable == null) throw new IllegalStateException("该标签不支持 NDEF 格式");
                formatable.connect();
                try {
                    formatable.format(message);
                } finally {
                    formatable.close();
                }
            }
            runOnUiThread(() -> {
                finishNfcWrite(tag);
                callJavaScript("window.__weikongNfcWriteSuccess", "");
            });
        } catch (Throwable error) {
            String messageText = error.getMessage();
            runOnUiThread(() -> {
                stopNfcWrite();
                callJavaScript("window.__weikongNfcWriteError", messageText == null ? "NFC 写入失败，请重新贴近标签" : messageText);
            });
        }
    }

    private void finishNfcWrite(Tag tag) {
        nfcWriteSessionActive = false;
        nfcWritePayload = null;
        pendingNfcWriteTag = null;
        nfcWriteInProgress.set(false);
        boolean waitingForRemoval = false;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N && nfcAdapter != null) {
            try {
                waitingForRemoval = nfcAdapter.ignore(tag, 1000, () ->
                    runOnUiThread(this::disableNfcReaderModeIfIdle), null);
            } catch (Throwable ignored) {}
        }
        if (!waitingForRemoval && webView != null) {
            // Android 6 has no NfcAdapter.ignore(). Keep reader mode briefly so the
            // system does not immediately rediscover the freshly written URI.
            webView.postDelayed(this::disableNfcReaderModeIfIdle, 1800);
        }
    }

    private void disableNfcReaderModeIfIdle() {
        // A new write may have started before the previous tag was removed.
        if (nfcWriteSessionActive || nfcWritePayload != null || nfcAdapter == null) return;
        try { nfcAdapter.disableReaderMode(this); } catch (Throwable ignored) {}
    }

    private void stopNfcWrite() {
        nfcWriteSessionActive = false;
        nfcWritePayload = null;
        pendingNfcWriteTag = null;
        nfcWriteInProgress.set(false);
        if (nfcAdapter != null) {
            try { nfcAdapter.disableReaderMode(this); } catch (Throwable ignored) {}
        }
    }

    private void handleNfcIntent(Intent intent) {
        if (intent == null) return;
        Uri data = intent.getData();
        if (data == null || !"weikong".equalsIgnoreCase(data.getScheme()) || !"gateway-bind".equalsIgnoreCase(data.getHost())) return;
        pendingNfcPayload = data.toString();
        callJavaScript("window.__weikongNfcRead", pendingNfcPayload);
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
        handleNfcIntent(intent);
        if ("com.weikong.iotcloud.OPEN_ALARMS".equals(intent.getAction())) {
            pendingOpenAlarms = false;
            callJavaScript("window.__weikongOpenAlarms", "");
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (nfcWriteSessionActive) enableNfcReaderMode();
    }

    @Override
    protected void onPause() {
        if (nfcAdapter != null) {
            try { nfcAdapter.disableReaderMode(this); } catch (Throwable ignored) {}
        }
        super.onPause();
    }

    private void prepareQRCodeScan() {
        if (!getPackageManager().hasSystemFeature(PackageManager.FEATURE_CAMERA_ANY)) {
            callJavaScript("window.__weikongScanError", "当前设备没有可用相机，请手动输入绑定信息");
            return;
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M
            && checkSelfPermission(Manifest.permission.CAMERA) != PackageManager.PERMISSION_GRANTED) {
            scanPending = true;
            requestPermissions(new String[]{Manifest.permission.CAMERA}, CAMERA_PERMISSION_REQUEST);
            return;
        }
        launchQRCodeScan();
    }

    private void launchQRCodeScan() {
        try {
            new IntentIntegrator(MainActivity.this)
                .setCaptureActivity(GatewayCaptureActivity.class)
                .setDesiredBarcodeFormats("QR_CODE")
                .setPrompt("将网关绑定二维码放入取景框")
                .setBeepEnabled(false)
                .setBarcodeImageEnabled(false)
                .setOrientationLocked(true)
                .initiateScan();
        } catch (Throwable error) {
            callJavaScript("window.__weikongScanError", "无法打开相机扫码，请检查相机权限或手动输入绑定信息");
        }
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode != CAMERA_PERMISSION_REQUEST) return;
        boolean shouldScan = scanPending;
        scanPending = false;
        if (shouldScan && grantResults.length > 0 && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
            launchQRCodeScan();
        } else {
            callJavaScript("window.__weikongScanError", "相机权限未开启，请允许相机权限后重试或手动输入绑定信息");
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        IntentResult result = IntentIntegrator.parseActivityResult(requestCode, resultCode, data);
        if (result == null) {
            super.onActivityResult(requestCode, resultCode, data);
            return;
        }
        if (result.getContents() == null) {
            String error = data == null ? null : data.getStringExtra("WEIKONG_SCAN_ERROR");
            callJavaScript("window.__weikongScanError", error == null ? "已取消扫码" : error);
        } else {
            callJavaScript("window.__weikongScanResult", result.getContents());
        }
    }

    private void callJavaScript(String callback, String value) {
        if (webView == null) return;
        String script = callback + " && " + callback + "(" + JSONObject.quote(value) + ")";
        webView.post(() -> webView.evaluateJavascript(script, null));
    }

    @Override
    public void onBackPressed() {
        if (webView != null && webView.canGoBack()) {
            webView.goBack();
        } else {
            super.onBackPressed();
        }
    }

    @Override
    protected void onDestroy() {
        stopNfcWrite();
        if (webView != null) {
            webView.destroy();
        }
        super.onDestroy();
    }
}
