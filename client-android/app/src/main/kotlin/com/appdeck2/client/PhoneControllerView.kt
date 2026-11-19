package com.appdeck2.client

import android.annotation.SuppressLint
import android.content.Context
import android.util.AttributeSet
import android.view.LayoutInflater
import android.view.MotionEvent
import android.webkit.WebView
import android.widget.Button
import android.widget.FrameLayout

class PhoneControllerView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null,
    defStyleAttr: Int = 0
) : FrameLayout(context, attrs, defStyleAttr) {

    companion object {
        private const val SCROLL_SCALE = 2.0f
        private const val TAP_THRESHOLD_PX = 20
        private const val TAP_THRESHOLD_MS = 250L
    }

    var tvWebView: WebView? = null

    private val touchpadArea: FrameLayout
    private val btnUp: Button
    private val btnDown: Button
    private val btnLeft: Button
    private val btnRight: Button
    private val btnA: Button
    private val btnB: Button

    private var touchStartX = 0f
    private var touchStartY = 0f
    private var touchStartTime = 0L
    private var lastTouchX = 0f
    private var lastTouchY = 0f

    init {
        LayoutInflater.from(context).inflate(R.layout.phone_controller, this, true)

        touchpadArea = findViewById(R.id.touchpadArea)
        btnUp = findViewById(R.id.btnUp)
        btnDown = findViewById(R.id.btnDown)
        btnLeft = findViewById(R.id.btnLeft)
        btnRight = findViewById(R.id.btnRight)
        btnA = findViewById(R.id.btnA)
        btnB = findViewById(R.id.btnB)

        setupTouchpad()
        setupDpad()
        setupActionButtons()
    }

    @SuppressLint("ClickableViewAccessibility")
    private fun setupTouchpad() {
        touchpadArea.setOnTouchListener { _, event ->
            when (event.action) {
                MotionEvent.ACTION_DOWN -> {
                    touchStartX = event.x
                    touchStartY = event.y
                    lastTouchX = event.x
                    lastTouchY = event.y
                    touchStartTime = System.currentTimeMillis()
                    true
                }
                MotionEvent.ACTION_MOVE -> {
                    val dx = (event.x - lastTouchX) * SCROLL_SCALE
                    val dy = (event.y - lastTouchY) * SCROLL_SCALE
                    lastTouchX = event.x
                    lastTouchY = event.y
                    tvWebView?.evaluateJavascript(
                        "window.scrollBy(${dx.toInt()}, ${dy.toInt()})", null
                    )
                    true
                }
                MotionEvent.ACTION_UP -> {
                    val totalDx = Math.abs(event.x - touchStartX)
                    val totalDy = Math.abs(event.y - touchStartY)
                    val elapsed = System.currentTimeMillis() - touchStartTime
                    if (totalDx < TAP_THRESHOLD_PX && totalDy < TAP_THRESHOLD_PX && elapsed < TAP_THRESHOLD_MS) {
                        // Tap — click focused element on TV
                        dispatchKeyToTv("Enter")
                    }
                    true
                }
                else -> false
            }
        }
    }

    private fun setupDpad() {
        btnUp.setOnClickListener { dispatchKeyToTv("ArrowUp") }
        btnDown.setOnClickListener { dispatchKeyToTv("ArrowDown") }
        btnLeft.setOnClickListener { dispatchKeyToTv("ArrowLeft") }
        btnRight.setOnClickListener { dispatchKeyToTv("ArrowRight") }
    }

    private fun setupActionButtons() {
        btnA.setOnClickListener { dispatchKeyToTv("Enter") }
        btnB.setOnClickListener { dispatchKeyToTv("Escape") }
    }

    private fun dispatchKeyToTv(key: String) {
        val js = """
            (function() {
                var el = document.activeElement || document.body;
                el.dispatchEvent(new KeyboardEvent('keydown', {key: '$key', bubbles: true}));
                el.dispatchEvent(new KeyboardEvent('keyup', {key: '$key', bubbles: true}));
            })()
        """.trimIndent()
        tvWebView?.evaluateJavascript(js, null)
    }
}
