# Follow Gate QRCode Display Fix

## Issue Description
The Follow Required Gate (H5_026) was displaying the text "QR Code" instead of an actual QRCode image when a user scanned the Wails QRCode without subscribing to the WeChat Official Account.

## Root Cause
1. **Field Name Inconsistency**: The backend was returning `follow_url` in the bind-device 403 error response, but `oa_qrcode_url` in the check-follow endpoint response.
2. **Frontend Missing Field Check**: The frontend bind-device error handler only checked for `data.data?.follow_url`, but not `oa_qrcode_url` (unlike the check-follow handler which checked both).
3. **Missing Logging**: No warning was logged when the QR URL was missing, making debugging difficult.

## Changes Made

### Frontend (h5/js/payment.js)
1. **Line 631-638**: Updated bind-device 403 error handler to check for both field names
   ```javascript
   // OLD: const followUrl = data.data?.follow_url;
   // NEW: const followUrl = data.data?.oa_qrcode_url || data.data?.follow_url;
   ```
   - Now consistent with checkFollowStatus (line 382)
   - Added logging for extracted URL and warning if missing

2. **Line 511-516**: Added warning logging in showFollowGate
   ```javascript
   console.log('[H5] showFollowGate called, oaQrcodeUrl:', this.oaQrcodeUrl);
   if (!this.oaQrcodeUrl) {
       console.warn('[H5] WARNING: No OA QR code URL configured...');
   }
   ```

### Backend (handlers/client/auth.go)
**Line 536-548**: Updated bind-device 403 response to include both field names
```go
"data": gin.H{
    "oa_qrcode_url": followURL,  // Primary field (consistent with check-follow)
    "follow_url":    followURL,  // Deprecated, kept for backward compatibility
},
```

### CODEMAP (memory/codemap_h5.md)
- Updated OAuth Flow section (lines 115-128) with detailed fix notes
- Updated API Integration section (lines 284-287) with 403 error response documentation

## How It Works Now
1. User scans QR without following OA
2. Backend returns 403 with both `oa_qrcode_url` and `follow_url` fields
3. Frontend checks both field names and extracts the URL
4. If URL is found: displays actual QR code image
5. If URL is missing: logs warning and shows fallback SVG with "QR Code" text
6. If image fails to load: shows "二维码加载失败" fallback message

## Configuration Required
For the QR code to display properly, the backend `.env` file must have:
```
WECHAT_OA_FOLLOW_URL=https://example.com/path/to/qrcode.jpg
```

This URL should point to a valid QR code image that users can long-press in WeChat to identify and follow the Official Account.

## Testing Checklist
- [ ] Set `WECHAT_OA_FOLLOW_URL` in backend `.env`
- [ ] Restart backend server
- [ ] Scan Wails client QR code as a user who hasn't followed the OA
- [ ] Verify: actual QR code image displays (not "QR Code" text)
- [ ] Verify: console logs show the extracted QR URL
- [ ] Test: long-press QR code in WeChat to verify it's recognizable
- [ ] Test: follow OA, tap retry button, verify gate closes and login proceeds

## Related Files
- Frontend: `/opt/test/backend/h5/js/payment.js` (lines 631-638, 511-516)
- Backend: `/opt/test/backend/handlers/client/auth.go` (lines 536-548)
- Config: `/opt/test/backend/config/config.go` (line 99)
- Service: `/opt/test/backend/services/wechat_oa_service.go` (line 229-232)
- CODEMAP: `/opt/test/.claude/memory/codemap_h5.md`
