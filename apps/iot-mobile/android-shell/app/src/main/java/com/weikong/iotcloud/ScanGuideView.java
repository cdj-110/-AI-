package com.weikong.iotcloud;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.LinearGradient;
import android.graphics.Paint;
import android.graphics.RectF;
import android.graphics.Shader;
import android.util.AttributeSet;
import android.view.View;

/** Draws a lightweight blue corner frame and an animated scan line. */
public class ScanGuideView extends View {
    private final Paint cornerPaint = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final Paint linePaint = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final float density;
    private long animationStartedAt;

    public ScanGuideView(Context context, AttributeSet attrs) {
        super(context, attrs);
        density = getResources().getDisplayMetrics().density;
        cornerPaint.setColor(Color.rgb(22, 119, 255));
        cornerPaint.setStyle(Paint.Style.STROKE);
        cornerPaint.setStrokeWidth(dp(4));
        cornerPaint.setStrokeCap(Paint.Cap.ROUND);
        animationStartedAt = System.currentTimeMillis();
        setLayerType(View.LAYER_TYPE_SOFTWARE, null);
    }

    @Override
    protected void onDraw(Canvas canvas) {
        super.onDraw(canvas);
        float size = Math.min(getWidth() - dp(52), dp(280));
        float left = (getWidth() - size) / 2f;
        float top = (getHeight() - size) / 2f;
        RectF frame = new RectF(left, top, left + size, top + size);
        float corner = dp(27);

        drawCorner(canvas, frame.left, frame.top, corner, 1, 1);
        drawCorner(canvas, frame.right, frame.top, corner, -1, 1);
        drawCorner(canvas, frame.left, frame.bottom, corner, 1, -1);
        drawCorner(canvas, frame.right, frame.bottom, corner, -1, -1);

        float progress = ((System.currentTimeMillis() - animationStartedAt) % 2200L) / 2200f;
        float y = frame.top + dp(14) + (frame.height() - dp(28)) * progress;
        linePaint.setShader(new LinearGradient(
            frame.left + dp(18), y, frame.right - dp(18), y,
            new int[]{Color.TRANSPARENT, Color.rgb(67, 155, 255), Color.TRANSPARENT},
            null, Shader.TileMode.CLAMP
        ));
        linePaint.setStrokeWidth(dp(2));
        linePaint.setShadowLayer(dp(7), 0, 0, Color.rgb(22, 119, 255));
        canvas.drawLine(frame.left + dp(18), y, frame.right - dp(18), y, linePaint);
        postInvalidateDelayed(16);
    }

    private void drawCorner(Canvas canvas, float x, float y, float length, int dx, int dy) {
        canvas.drawLine(x, y, x + length * dx, y, cornerPaint);
        canvas.drawLine(x, y, x, y + length * dy, cornerPaint);
    }

    private float dp(float value) {
        return value * density;
    }
}
