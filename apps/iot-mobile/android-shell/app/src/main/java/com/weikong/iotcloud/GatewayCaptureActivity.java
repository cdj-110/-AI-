package com.weikong.iotcloud;

import android.os.Bundle;
import android.view.View;
import android.view.ViewGroup;
import android.widget.TextView;

import com.journeyapps.barcodescanner.CaptureActivity;
import com.journeyapps.barcodescanner.DecoratedBarcodeView;

/** Portrait scanner with a gateway-specific visual shell. */
public class GatewayCaptureActivity extends CaptureActivity {
    private DecoratedBarcodeView scannerView;
    private boolean torchEnabled;

    @Override
    protected DecoratedBarcodeView initializeContent() {
        setContentView(R.layout.gateway_capture);
        return findViewById(com.google.zxing.client.android.R.id.zxing_barcode_scanner);
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        scannerView = findViewById(com.google.zxing.client.android.R.id.zxing_barcode_scanner);
        View header = findViewById(R.id.scanner_header);
        if (header != null) {
            final int baseHeight = Math.round(76 * getResources().getDisplayMetrics().density);
            header.setOnApplyWindowInsetsListener((view, insets) -> {
                int topInset = insets.getSystemWindowInsetTop();
                view.setPadding(view.getPaddingLeft(), topInset, view.getPaddingRight(), view.getPaddingBottom());
                ViewGroup.LayoutParams params = view.getLayoutParams();
                params.height = baseHeight + topInset;
                view.setLayoutParams(params);
                return insets;
            });
            header.requestApplyInsets();
        }
    }

    public void closeScanner(View view) {
        finish();
    }

    public void toggleTorch(View view) {
        if (scannerView == null) return;
        torchEnabled = !torchEnabled;
        if (torchEnabled) {
            scannerView.setTorchOn();
        } else {
            scannerView.setTorchOff();
        }
        if (view instanceof TextView) {
            ((TextView) view).setText(torchEnabled ? "关闭手电筒" : "打开手电筒");
            view.setSelected(torchEnabled);
        }
    }
}
